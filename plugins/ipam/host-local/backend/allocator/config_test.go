// Copyright 2016 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package allocator

import (
	"net"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/containernetworking/cni/pkg/types"
)

var _ = Describe("IPAM config", func() {
	It("Should parse an old-style config", func() {
		input := `{
	"cniVersion": "0.3.1",
	"name": "mynet",
	"type": "ipvlan",
	"master": "foo0",
	"ipam": {
		"type": "host-local",
		"subnet": "10.1.2.0/24",
		"rangeStart": "10.1.2.9",
		"rangeEnd": "10.1.2.20",
		"gateway": "10.1.2.30"
	}
}`
		conf, version, err := LoadIPAMConfig([]byte(input), "")
		Expect(err).NotTo(HaveOccurred())
		Expect(version).Should(Equal("0.3.1"))

		Expect(conf).To(Equal(&IPAMConfig{
			Name: "mynet",
			Type: "host-local",
			Ranges: []RangeSet{
				{
					{
						RangeStart: net.IP{10, 1, 2, 9},
						RangeEnd:   net.IP{10, 1, 2, 20},
						Gateway:    net.IP{10, 1, 2, 30},
						Subnet: types.IPNet{
							IP:   net.IP{10, 1, 2, 0},
							Mask: net.CIDRMask(24, 32),
						},
					},
				},
			},
		}))
	})

	It("Should parse a new-style config", func() {
		input := `{
			"cniVersion": "0.3.1",
			"name": "mynet",
			"type": "ipvlan",
			"master": "foo0",
			"ipam": {
				"type": "host-local",
				"ranges": [
					[
						{
							"subnet": "10.1.2.0/24",
							"rangeStart": "10.1.2.9",
							"rangeEnd": "10.1.2.20",
							"gateway": "10.1.2.30"
						},
						{
							"subnet": "10.1.4.0/24"
						}
					],
					[{
						"subnet": "11.1.2.0/24",
						"rangeStart": "11.1.2.9",
						"rangeEnd": "11.1.2.20",
						"gateway": "11.1.2.30"
					}]
				]
			}
		}`
		conf, version, err := LoadIPAMConfig([]byte(input), "")
		Expect(err).NotTo(HaveOccurred())
		Expect(version).Should(Equal("0.3.1"))

		Expect(conf).To(Equal(&IPAMConfig{
			Name: "mynet",
			Type: "host-local",
			Ranges: []RangeSet{
				{
					{
						RangeStart: net.IP{10, 1, 2, 9},
						RangeEnd:   net.IP{10, 1, 2, 20},
						Gateway:    net.IP{10, 1, 2, 30},
						Subnet: types.IPNet{
							IP:   net.IP{10, 1, 2, 0},
							Mask: net.CIDRMask(24, 32),
						},
					},
					{
						RangeStart: net.IP{10, 1, 4, 1},
						RangeEnd:   net.IP{10, 1, 4, 254},
						Gateway:    net.IP{10, 1, 4, 1},
						Subnet: types.IPNet{
							IP:   net.IP{10, 1, 4, 0},
							Mask: net.CIDRMask(24, 32),
						},
					},
				},
				{
					{
						RangeStart: net.IP{11, 1, 2, 9},
						RangeEnd:   net.IP{11, 1, 2, 20},
						Gateway:    net.IP{11, 1, 2, 30},
						Subnet: types.IPNet{
							IP:   net.IP{11, 1, 2, 0},
							Mask: net.CIDRMask(24, 32),
						},
					},
				},
			},
		}))
	})

	It("Should parse a mixed config with runtime args", func() {
		input := `{
			"cniVersion": "0.3.1",
			"name": "mynet",
			"type": "ipvlan",
			"master": "foo0",
			"runtimeConfig": {
				"irrelevant": "a",
				"ipRanges": [
					[{ "subnet": "12.1.3.0/24" }]
				]
			},
			"ipam": {
				"type": "host-local",
				"subnet": "10.1.2.0/24",
				"rangeStart": "10.1.2.9",
				"rangeEnd": "10.1.2.20",
				"gateway": "10.1.2.30",
				"ranges": [[
					{
						"subnet": "11.1.2.0/24",
						"rangeStart": "11.1.2.9",
						"rangeEnd": "11.1.2.20",
						"gateway": "11.1.2.30"
					}
				]]
			}
		}`
		conf, version, err := LoadIPAMConfig([]byte(input), "")
		Expect(err).NotTo(HaveOccurred())
		Expect(version).Should(Equal("0.3.1"))

		Expect(conf).To(Equal(&IPAMConfig{
			Name: "mynet",
			Type: "host-local",
			Ranges: []RangeSet{
				{ // The RuntimeConfig should always be first
					{
						RangeStart: net.IP{12, 1, 3, 1},
						RangeEnd:   net.IP{12, 1, 3, 254},
						Gateway:    net.IP{12, 1, 3, 1},
						Subnet: types.IPNet{
							IP:   net.IP{12, 1, 3, 0},
							Mask: net.CIDRMask(24, 32),
						},
					},
				},
				{
					{
						RangeStart: net.IP{10, 1, 2, 9},
						RangeEnd:   net.IP{10, 1, 2, 20},
						Gateway:    net.IP{10, 1, 2, 30},
						Subnet: types.IPNet{
							IP:   net.IP{10, 1, 2, 0},
							Mask: net.CIDRMask(24, 32),
						},
					},
				},
				{
					{
						RangeStart: net.IP{11, 1, 2, 9},
						RangeEnd:   net.IP{11, 1, 2, 20},
						Gateway:    net.IP{11, 1, 2, 30},
						Subnet: types.IPNet{
							IP:   net.IP{11, 1, 2, 0},
							Mask: net.CIDRMask(24, 32),
						},
					},
				},
			},
		}))
	})

	Context("Should parse CNI_ARGS env", func() {
		It("without prefix", func() {
			input := `{
			"cniVersion": "0.3.1",
			"name": "mynet",
			"type": "ipvlan",
			"master": "foo0",
			"ipam": {
				"type": "host-local",
				"ranges": [[
					{
						"subnet": "10.1.2.0/24",
						"rangeStart": "10.1.2.9",
						"rangeEnd": "10.1.2.20",
						"gateway": "10.1.2.30"
					}
				]]
			}
		}`

			envArgs := "IP=10.1.2.10"

			conf, _, err := LoadIPAMConfig([]byte(input), envArgs)
			Expect(err).NotTo(HaveOccurred())
			Expect(conf.IPArgs).To(Equal([]net.IP{{10, 1, 2, 10}}))
		})

		It("with prefix", func() {
			input := `{
			"cniVersion": "0.3.1",
			"name": "mynet",
			"type": "ipvlan",
			"master": "foo0",
			"ipam": {
				"type": "host-local",
				"ranges": [[
					{
						"subnet": "10.1.2.0/24",
						"rangeStart": "10.1.2.9",
						"rangeEnd": "10.1.2.20",
						"gateway": "10.1.2.30"
					}
				]]
			}
		}`

			envArgs := "IP=10.1.2.11/24"

			conf, _, err := LoadIPAMConfig([]byte(input), envArgs)
			Expect(err).NotTo(HaveOccurred())
			Expect(conf.IPArgs).To(Equal([]net.IP{{10, 1, 2, 11}}))
		})
	})

	Context("Should parse config args", func() {
		It("without prefix", func() {
			input := `{
			"cniVersion": "0.3.1",
			"name": "mynet",
			"type": "ipvlan",
			"master": "foo0",
			"args": {
				"cni": {
					"ips": [ "10.1.2.11", "11.11.11.11", "2001:db8:1::11"]
				}
			},
			"ipam": {
				"type": "host-local",
				"ranges": [
					[{
						"subnet": "10.1.2.0/24",
						"rangeStart": "10.1.2.9",
						"rangeEnd": "10.1.2.20",
						"gateway": "10.1.2.30"
					}],
					[{
						"subnet": "11.1.2.0/24",
						"rangeStart": "11.1.2.9",
						"rangeEnd": "11.1.2.20",
						"gateway": "11.1.2.30"
					}],
					[{
						"subnet": "2001:db8:1::/64"
					}]
				]
			}
		}`

			envArgs := "IP=10.1.2.10"

			conf, _, err := LoadIPAMConfig([]byte(input), envArgs)
			Expect(err).NotTo(HaveOccurred())
			Expect(conf.IPArgs).To(Equal([]net.IP{
				{10, 1, 2, 10},
				{10, 1, 2, 11},
				{11, 11, 11, 11},
				net.ParseIP("2001:db8:1::11"),
			}))
		})

		It("with prefix", func() {
			input := `{
			"cniVersion": "0.3.1",
			"name": "mynet",
			"type": "ipvlan",
			"master": "foo0",
			"args": {
				"cni": {
					"ips": [ "10.1.2.11/24", "11.11.11.11/24", "2001:db8:1::11/64"]
				}
			},
			"ipam": {
				"type": "host-local",
				"ranges": [
					[{
						"subnet": "10.1.2.0/24",
						"rangeStart": "10.1.2.9",
						"rangeEnd": "10.1.2.20",
						"gateway": "10.1.2.30"
					}],
					[{
						"subnet": "11.1.2.0/24",
						"rangeStart": "11.1.2.9",
						"rangeEnd": "11.1.2.20",
						"gateway": "11.1.2.30"
					}],
					[{
						"subnet": "2001:db8:1::/64"
					}]
				]
			}
		}`

			envArgs := "IP=10.1.2.10/24"

			conf, _, err := LoadIPAMConfig([]byte(input), envArgs)
			Expect(err).NotTo(HaveOccurred())
			Expect(conf.IPArgs).To(Equal([]net.IP{
				{10, 1, 2, 10},
				{10, 1, 2, 11},
				{11, 11, 11, 11},
				net.ParseIP("2001:db8:1::11"),
			}))
		})
	})

	It("Should detect overlap between rangesets", func() {
		input := `{
			"cniVersion": "0.3.1",
			"name": "mynet",
			"type": "ipvlan",
			"master": "foo0",
			"ipam": {
				"type": "host-local",
				"ranges": [
					[
						{"subnet": "10.1.2.0/24"},
						{"subnet": "10.2.2.0/24"}
					],
					[
						{ "subnet": "10.1.4.0/24"},
						{ "subnet": "10.1.6.0/24"},
						{ "subnet": "10.1.8.0/24"},
						{ "subnet": "10.1.2.0/24"}
					]
				]
			}
		}`
		_, _, err := LoadIPAMConfig([]byte(input), "")
		Expect(err).To(MatchError("range set 0 overlaps with 1"))
	})

	It("Should detect overlap within rangeset", func() {
		input := `{
			"cniVersion": "0.3.1",
			"name": "mynet",
			"type": "ipvlan",
			"master": "foo0",
			"ipam": {
				"type": "host-local",
				"ranges": [
					[
						{ "subnet": "10.1.0.0/22" },
						{ "subnet": "10.1.2.0/24" }
					]
				]
			}
		}`
		_, _, err := LoadIPAMConfig([]byte(input), "")
		Expect(err).To(MatchError("invalid range set 0: subnets 10.1.0.1-10.1.3.254 and 10.1.2.1-10.1.2.254 overlap"))
	})

	It("should error on rangesets with different families", func() {
		input := `{
			"cniVersion": "0.3.1",
			"name": "mynet",
			"type": "ipvlan",
			"master": "foo0",
			"ipam": {
				"type": "host-local",
				"ranges": [
					[
						{ "subnet": "10.1.0.0/22" },
						{ "subnet": "2001:db8:5::/64" }
					]
				]
			}
		}`
		_, _, err := LoadIPAMConfig([]byte(input), "")
		Expect(err).To(MatchError("invalid range set 0: mixed address families"))
	})

	It("Should should error on too many ranges", func() {
		input := `{
				"cniVersion": "0.2.0",
				"name": "mynet",
				"type": "ipvlan",
				"master": "foo0",
				"ipam": {
					"type": "host-local",
					"ranges": [
						[{"subnet": "10.1.2.0/24"}],
						[{"subnet": "11.1.2.0/24"}]
					]
				}
			}`
		_, _, err := LoadIPAMConfig([]byte(input), "")
		Expect(err).To(MatchError("CNI version 0.2.0 does not support more than 1 address per family"))
	})

	It("Should allow one v4 and v6 range for 0.2.0", func() {
		input := `{
				"cniVersion": "0.2.0",
				"name": "mynet",
				"type": "ipvlan",
				"master": "foo0",
				"ipam": {
					"type": "host-local",
					"ranges": [
						[{"subnet": "10.1.2.0/24"}],
						[{"subnet": "2001:db8:1::/48"}]
					]
				}
			}`
		_, _, err := LoadIPAMConfig([]byte(input), "")
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should parse custom IPs from runtime configuration", func() {
		input := `{
			"cniVersion": "0.3.1",
			"name": "mynet",
			"type": "ipvlan",
			"master": "foo0",
			"runtimeConfig": {
				"ips": ["192.168.0.1", "192.168.0.5/24", "2001:db8::1/64"]
			},
			"ipam": {
				"type": "host-local",
				"subnet": "10.1.2.0/24"
			}
		}`
		conf, version, err := LoadIPAMConfig([]byte(input), "")
		Expect(err).NotTo(HaveOccurred())
		Expect(version).Should(Equal("0.3.1"))

		Expect(conf.IPArgs).To(Equal([]net.IP{
			net.IPv4(192, 168, 0, 1).To4(),
			net.IPv4(192, 168, 0, 5).To4(),
			net.ParseIP("2001:db8::1"),
		}))
	})

	It("Should parse ipam ranges from a file", func() {
		// Create a temporary file with the ranges
		tempFile, err := os.CreateTemp("", "ranges.conf")
		Expect(err).NotTo(HaveOccurred())
		defer os.Remove(tempFile.Name())
		// Write the ranges to the file
		_, err = tempFile.WriteString(`{"subnet": "10.128.0.0/11", "rangeStart": "10.128.12.100", "rangeEnd": "10.128.12.254"}`)
		Expect(err).NotTo(HaveOccurred())

		// use the temp file as the range file
		input := `{
			"cniVersion": "0.3.1",
			"name": "mynet",
			"type": "macvlan",
			"master": "foo0",
			"ipam": {
				"type": "host-local",
				"rangeFromFile": "` + tempFile.Name() + `"
			}
		}`
		conf, version, err := LoadIPAMConfig([]byte(input), "")
		Expect(err).NotTo(HaveOccurred())
		Expect(version).Should(Equal("0.3.1"))

		rgFromFile := conf.Ranges[0][0]
		Expect(rgFromFile.Subnet).To(Equal(types.IPNet{IP: net.IPv4(10, 128, 0, 0).To4(), Mask: net.CIDRMask(11, 32)}))
		Expect(rgFromFile.RangeStart).To(Equal(net.IPv4(10, 128, 12, 100)))
		Expect(rgFromFile.RangeEnd).To(Equal(net.IPv4(10, 128, 12, 254)))
	})
})
