// Copyright 2018 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/vishvananda/netlink"

	"github.com/containernetworking/plugins/pkg/netlinksafe"
	"github.com/containernetworking/plugins/pkg/ns"
)

func TestTBF(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "plugins/meta/bandwidth")
}

var echoServerBinaryPath, echoClientBinaryPath string

var _ = SynchronizedBeforeSuite(func() []byte {
	serverBinaryPath, err := gexec.Build("github.com/containernetworking/plugins/pkg/testutils/echo/server")
	Expect(err).NotTo(HaveOccurred())
	clientBinaryPath, err := gexec.Build("github.com/containernetworking/plugins/pkg/testutils/echo/client")
	Expect(err).NotTo(HaveOccurred())
	return []byte(strings.Join([]string{serverBinaryPath, clientBinaryPath}, ","))
}, func(data []byte) {
	binaries := strings.Split(string(data), ",")
	echoServerBinaryPath = binaries[0]
	echoClientBinaryPath = binaries[1]
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	gexec.CleanupBuildArtifacts()
})

func startInNetNS(binPath string, netNS ns.NetNS) (*gexec.Session, error) {
	baseName := filepath.Base(netNS.Path())
	// we are relying on the netNS path living in /var/run/netns
	// where `ip netns exec` can find it
	cmd := exec.Command("ip", "netns", "exec", baseName, binPath)

	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	return session, err
}

func startEchoServerInNamespace(netNS ns.NetNS) (int, *gexec.Session) {
	session, err := startInNetNS(echoServerBinaryPath, netNS)
	Expect(err).NotTo(HaveOccurred())

	// wait for it to print it's address on stdout
	Eventually(session.Out).Should(gbytes.Say("\n"))
	_, portString, err := net.SplitHostPort(strings.TrimSpace(string(session.Out.Contents())))
	Expect(err).NotTo(HaveOccurred())

	port, err := strconv.Atoi(portString)
	Expect(err).NotTo(HaveOccurred())

	go func() {
		// print out echoserver output to ginkgo to capture any errors that might be occurring.
		io.Copy(GinkgoWriter, io.MultiReader(session.Out, session.Err))
	}()

	return port, session
}

func makeTCPClientInNS(netns string, address string, port int, numBytes int) {
	payload := bytes.Repeat([]byte{'a'}, numBytes)
	message := string(payload)

	var cmd *exec.Cmd
	if netns != "" {
		netns = filepath.Base(netns)
		cmd = exec.Command("ip", "netns", "exec", netns, echoClientBinaryPath, "--target", fmt.Sprintf("%s:%d", address, port), "--message", message)
	} else {
		cmd = exec.Command(echoClientBinaryPath, "--target", fmt.Sprintf("%s:%d", address, port), "--message", message)
	}
	cmd.Stdin = bytes.NewBuffer([]byte(message))
	cmd.Stderr = GinkgoWriter
	out, err := cmd.Output()

	Expect(err).NotTo(HaveOccurred())
	Expect(string(out)).To(Equal(message))
}

func createVeth(hostNs ns.NetNS, hostVethIfName string, containerNs ns.NetNS, containerVethIfName string, hostIP []byte, containerIP []byte, hostIfaceMTU int) {
	linkAttrs := netlink.NewLinkAttrs()
	linkAttrs.Name = hostVethIfName
	linkAttrs.Flags = net.FlagUp
	linkAttrs.MTU = hostIfaceMTU
	vethDeviceRequest := &netlink.Veth{
		LinkAttrs: linkAttrs,
		PeerName:  containerVethIfName,
	}

	err := hostNs.Do(func(_ ns.NetNS) error {
		if err := netlink.LinkAdd(vethDeviceRequest); err != nil {
			return fmt.Errorf("creating veth pair: %s", err)
		}

		containerVeth, err := netlinksafe.LinkByName(containerVethIfName)
		if err != nil {
			return fmt.Errorf("failed to find newly-created veth device %q: %v", containerVethIfName, err)
		}

		err = netlink.LinkSetNsFd(containerVeth, int(containerNs.Fd()))
		if err != nil {
			return fmt.Errorf("failed to move veth to container namespace: %s", err)
		}

		localAddr := &net.IPNet{
			IP:   hostIP,
			Mask: []byte{255, 255, 255, 255},
		}
		peerAddr := &net.IPNet{
			IP:   containerIP,
			Mask: []byte{255, 255, 255, 255},
		}
		addr, err := netlink.ParseAddr(localAddr.String())
		if err != nil {
			return fmt.Errorf("parsing address %s: %s", localAddr, err)
		}

		addr.Peer = peerAddr

		addr.Scope = int(netlink.SCOPE_LINK)
		hostVeth, err := netlinksafe.LinkByName(hostVethIfName)
		if err != nil {
			return fmt.Errorf("failed to find newly-created veth device %q: %v", containerVethIfName, err)
		}

		err = netlink.AddrAdd(hostVeth, addr)
		if err != nil {
			return fmt.Errorf("adding IP address %s: %s", localAddr, err)
		}

		return nil
	})
	Expect(err).NotTo(HaveOccurred())

	err = containerNs.Do(func(_ ns.NetNS) error {
		peerAddr := &net.IPNet{
			IP:   hostIP,
			Mask: []byte{255, 255, 255, 255},
		}
		localAddr := &net.IPNet{
			IP:   containerIP,
			Mask: []byte{255, 255, 255, 255},
		}
		addr, err := netlink.ParseAddr(localAddr.String())
		if err != nil {
			return fmt.Errorf("parsing address %s: %s", localAddr, err)
		}

		addr.Peer = peerAddr

		addr.Scope = int(netlink.SCOPE_LINK)
		containerVeth, err := netlinksafe.LinkByName(containerVethIfName)
		if err != nil {
			return fmt.Errorf("failed to find newly-created veth device %q: %v", containerVethIfName, err)
		}
		err = netlink.AddrAdd(containerVeth, addr)
		if err != nil {
			return fmt.Errorf("adding IP address %s: %s", localAddr, err)
		}

		return nil
	})

	Expect(err).NotTo(HaveOccurred())
}

func createVethInOneNs(netNS ns.NetNS, vethName, peerName string) {
	linkAttrs := netlink.NewLinkAttrs()
	linkAttrs.Name = vethName
	linkAttrs.Flags = net.FlagUp
	vethDeviceRequest := &netlink.Veth{
		LinkAttrs: linkAttrs,
		PeerName:  peerName,
	}

	err := netNS.Do(func(_ ns.NetNS) error {
		if err := netlink.LinkAdd(vethDeviceRequest); err != nil {
			return fmt.Errorf("failed to create veth pair: %v", err)
		}

		_, err := netlinksafe.LinkByName(peerName)
		if err != nil {
			return fmt.Errorf("failed to find newly-created veth device %q: %v", peerName, err)
		}
		return nil
	})
	Expect(err).NotTo(HaveOccurred())
}

func createMacvlan(netNS ns.NetNS, master, macvlanName string) {
	err := netNS.Do(func(_ ns.NetNS) error {
		m, err := netlinksafe.LinkByName(master)
		if err != nil {
			return fmt.Errorf("failed to lookup master %q: %v", master, err)
		}

		linkAttrs := netlink.NewLinkAttrs()
		linkAttrs.MTU = m.Attrs().MTU
		linkAttrs.Name = macvlanName
		linkAttrs.ParentIndex = m.Attrs().Index
		macvlanDeviceRequest := &netlink.Macvlan{
			LinkAttrs: linkAttrs,
			Mode:      netlink.MACVLAN_MODE_BRIDGE,
		}

		if err = netlink.LinkAdd(macvlanDeviceRequest); err != nil {
			return fmt.Errorf("failed to create macvlan device: %s", err)
		}

		_, err = netlinksafe.LinkByName(macvlanName)
		if err != nil {
			return fmt.Errorf("failed to find newly-created macvlan device %q: %v", macvlanName, err)
		}
		return nil
	})
	Expect(err).NotTo(HaveOccurred())
}
