// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of go-ra

package ra

import (
	"bytes"
	"os"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

func TestConfigParsers(t *testing.T) {
	yamlConf := `
interfaces:
  - name: net0
    raIntervalMilliseconds: 1000
  - name: net1
    raIntervalMilliseconds: 1000
`

	t.Run("ParseConfigYAMLFile", func(t *testing.T) {
		f, err := os.CreateTemp(".", "ra-test")
		require.NoError(t, err)
		defer os.Remove(f.Name())
		_, err = f.Write([]byte(yamlConf))
		require.NoError(t, err)
		c, err := ParseConfigYAMLFile(f.Name())
		require.NoError(t, err)
		require.NotNil(t, c)
		require.Len(t, c.Interfaces, 2)
		require.Equal(t, "net0", c.Interfaces[0].Name)
		require.Equal(t, 1000, c.Interfaces[0].RAIntervalMilliseconds)
		require.Equal(t, "net1", c.Interfaces[1].Name)
		require.Equal(t, 1000, c.Interfaces[1].RAIntervalMilliseconds)
	})

	jsonConf := `
{
	"interfaces": [
		{
			"name": "net0",
			"raIntervalMilliseconds": 1000
		},
		{
			"name": "net1",
			"raIntervalMilliseconds": 1000
		}
	]
}
`

	t.Run("ParseConfigJSON", func(t *testing.T) {
		c, err := ParseConfigJSON(bytes.NewBuffer([]byte(jsonConf)))
		require.NoError(t, err)
		require.NotNil(t, c)
		require.Len(t, c.Interfaces, 2)
		require.Equal(t, "net0", c.Interfaces[0].Name)
		require.Equal(t, 1000, c.Interfaces[0].RAIntervalMilliseconds)
		require.Equal(t, "net1", c.Interfaces[1].Name)
		require.Equal(t, 1000, c.Interfaces[1].RAIntervalMilliseconds)
	})

}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorField  string
		errorTag    string
	}{
		{
			name: "Valid Config",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
					},
					{
						Name:                   "net1",
						RAIntervalMilliseconds: 1000,
					},
				},
			},
			expectError: false,
		},
		{
			name: "Empty InterfaceConig",
			config: &Config{
				Interfaces: []*InterfaceConfig{},
			},
			expectError: false,
		},
		{
			name: "Nil InterfaceConig Element",
			config: &Config{
				Interfaces: []*InterfaceConfig{nil},
			},
			expectError: true,
			errorField:  "Name",
			errorTag:    "required",
		},
		{
			name: "Duplicated Interface Name",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
					},
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
					},
				},
			},
			expectError: true,
			errorField:  "Interfaces",
			errorTag:    "unique",
		},
		{
			name: "RAIntervalMilliseconds < 70",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 69,
					},
				},
			},
			expectError: true,
			errorField:  "RAIntervalMilliseconds",
			errorTag:    "gte",
		},
		{
			name: "RAIntervalMilliseconds > 1800000",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1800001,
					},
				},
			},
			expectError: true,
			errorField:  "RAIntervalMilliseconds",
			errorTag:    "lte",
		},
		{
			name: "CurrentHopLimit < 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						CurrentHopLimit:        -1,
					},
				},
			},
			expectError: true,
			errorField:  "CurrentHopLimit",
			errorTag:    "gte",
		},
		{
			name: "CurrentHopLimit > 255",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						CurrentHopLimit:        256,
					},
				},
			},
			expectError: true,
			errorField:  "CurrentHopLimit",
			errorTag:    "lte",
		},
		{
			name: "RouterLifetimeSeconds < 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						RouterLifetimeSeconds:  -1,
					},
				},
			},
			expectError: true,
			errorField:  "RouterLifetimeSeconds",
			errorTag:    "gte",
		},
		{
			name: "RouterLifetimeSeconds > 65535",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						RouterLifetimeSeconds:  65536,
					},
				},
			},
			expectError: true,
			errorField:  "RouterLifetimeSeconds",
			errorTag:    "lte",
		},
		{
			name: "ReachableTimeMilliseconds < 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                      "net0",
						RAIntervalMilliseconds:    1000,
						ReachableTimeMilliseconds: -1,
					},
				},
			},
			expectError: true,
			errorField:  "ReachableTimeMilliseconds",
			errorTag:    "gte",
		},
		{
			name: "ReachableTimeMilliseconds > 4294967295",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                      "net0",
						RAIntervalMilliseconds:    1000,
						ReachableTimeMilliseconds: 4294967296,
					},
				},
			},
			expectError: true,
			errorField:  "ReachableTimeMilliseconds",
			errorTag:    "lte",
		},
		{
			name: "RetransmitTimeMilliseconds < 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                       "net0",
						RAIntervalMilliseconds:     1000,
						RetransmitTimeMilliseconds: -1,
					},
				},
			},
			expectError: true,
			errorField:  "RetransmitTimeMilliseconds",
			errorTag:    "gte",
		},
		{
			name: "RetransmitTimeMilliseconds > 4294967295",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                       "net0",
						RAIntervalMilliseconds:     1000,
						RetransmitTimeMilliseconds: 4294967296,
					},
				},
			},
			expectError: true,
			errorField:  "RetransmitTimeMilliseconds",
			errorTag:    "lte",
		},
		{
			name: "MTU > 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						MTU:                    -1,
					},
				},
			},
			expectError: true,
			errorField:  "MTU",
			errorTag:    "gte",
		},
		{
			name: "MTU > 4294967295",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						MTU:                    4294967296,
					},
				},
			},
			expectError: true,
			errorField:  "MTU",
			errorTag:    "lte",
		},

		// PrefixConfig
		{
			name: "Nil PrefixConfig",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Prefixes:               nil,
					},
				},
			},
			expectError: false,
		},
		{
			name: "Empty PrefixConfig",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Prefixes:               []*PrefixConfig{},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Nil PrefixConfig Element",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Prefixes:               []*PrefixConfig{nil},
					},
				},
			},
			expectError: true,
			errorField:  "Prefix",
			errorTag:    "required",
		},
		{
			name: "No Prefix",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Prefixes: []*PrefixConfig{
							{
								OnLink: true,
							},
						},
					},
				},
			},
			expectError: true,
			errorField:  "Prefix",
			errorTag:    "required",
		},
		{
			name: "Overlapping Prefix",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Prefixes: []*PrefixConfig{
							{
								Prefix: "2001:db8::/32",
							},
							{
								Prefix: "2001:db8::/64",
							},
						},
					},
				},
			},
			expectError: true,
			errorField:  "Prefixes",
			errorTag:    "non_overlapping_prefix",
		},
		{
			name: "ValidLifetimeSeconds = 4294967295",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Prefixes: []*PrefixConfig{
							{
								Prefix:               "2001:db8::/64",
								ValidLifetimeSeconds: ptr.To(4294967295),
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "ValidLifetimeSeconds < 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Prefixes: []*PrefixConfig{
							{
								Prefix:               "2001:db8::/64",
								ValidLifetimeSeconds: ptr.To(-1),
							},
						},
					},
				},
			},
			expectError: true,
			errorField:  "ValidLifetimeSeconds",
			errorTag:    "gte",
		},
		{
			name: "ValidLifetimeSeconds > 4294967295",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Prefixes: []*PrefixConfig{
							{
								Prefix:               "2001:db8::/64",
								ValidLifetimeSeconds: ptr.To(4294967296),
							},
						},
					},
				},
			},
			expectError: true,
			errorField:  "ValidLifetimeSeconds",
			errorTag:    "lte",
		},
		{
			name: "PreferredLifetimeSeconds = 4294967295",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Prefixes: []*PrefixConfig{
							{
								Prefix:                   "2001:db8::/64",
								ValidLifetimeSeconds:     ptr.To(4294967295), // PreferredLifetimeSeconds must be less than ValidLifetimeSeconds
								PreferredLifetimeSeconds: ptr.To(4294967295),
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "PreferredLifetimeSeconds < 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Prefixes: []*PrefixConfig{
							{
								Prefix:                   "2001:db8::/64",
								PreferredLifetimeSeconds: ptr.To(-1),
							},
						},
					},
				},
			},
			expectError: true,
			errorField:  "PreferredLifetimeSeconds",
			errorTag:    "gte",
		},
		{
			name: "PreferredLifetimeSeconds > 4294967295",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Prefixes: []*PrefixConfig{
							{
								Prefix:                   "2001:db8::/64",
								ValidLifetimeSeconds:     ptr.To(4294967296),
								PreferredLifetimeSeconds: ptr.To(4294967296),
							},
						},
					},
				},
			},
			expectError: true,
			// PreferredLifetimeSeconds must be less than
			// ValidLifetimeSeconds, but ValdateLifetimeSeconds
			// must be <= 4294967295, so it's impossible to specify
			// PreferredLifetimeSeconds > 4294967295 actually.
			errorField: "ValidLifetimeSeconds",
			errorTag:   "lte",
		},
		{
			name: "ValidLifetimeSeconds < PreferredLifetimeSeconds",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Prefixes: []*PrefixConfig{
							{
								Prefix:                   "2001:db8::/64",
								ValidLifetimeSeconds:     ptr.To(100),
								PreferredLifetimeSeconds: ptr.To(101),
							},
						},
					},
				},
			},
			expectError: true,
			errorField:  "PreferredLifetimeSeconds",
			errorTag:    "ltefield",
		},
		{
			name: "Preference low && RouterLifetimeSeconds != 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Preference:             "low",
						RouterLifetimeSeconds:  1,
					},
				},
			},
		},
		{
			name: "Preference medium && RouterLifetimeSeconds != 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Preference:             "medium",
						RouterLifetimeSeconds:  1,
					},
				},
			},
		},
		{
			name: "Preference high && RouterLifetimeSeconds != 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Preference:             "high",
						RouterLifetimeSeconds:  1,
					},
				},
			},
		},
		{
			name: "Preference foo && RouterLifetimeSeconds != 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Preference:             "foo",
						RouterLifetimeSeconds:  1,
					},
				},
			},
			expectError: true,
			errorField:  "Preference",
			errorTag:    "oneof",
		},
		{
			name: "Preference == low && RouterLifetimeSeconds == 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Preference:             "low",
						RouterLifetimeSeconds:  0,
					},
				},
			},
			expectError: true,
			errorField:  "Preference",
			errorTag:    "eq_if medium RouterLifetimeSeconds 0",
		},
		{
			name: "Preference == <empty> && RouterLifetimeSeconds == 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						RouterLifetimeSeconds:  0,
					},
				},
			},
		},

		// RouteConfig
		{
			name: "Nil RouteConfig",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Routes:                 nil,
					},
				},
			},
			expectError: false,
		},
		{
			name: "Empty RouteConfig",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Routes:                 []*RouteConfig{},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Nil RouteConfig Element",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Routes:                 []*RouteConfig{nil},
					},
				},
			},
			expectError: true,
			errorField:  "Prefix",
			errorTag:    "required",
		},
		{
			name: "No Prefix",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Routes: []*RouteConfig{
							{},
						},
					},
				},
			},
			expectError: true,
			errorField:  "Prefix",
			errorTag:    "required",
		},
		{
			name: "No LifetimeSeconds",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Routes: []*RouteConfig{
							{
								Prefix: "2001:db8::/64",
							},
						},
					},
				},
			},
			expectError: true,
			errorField:  "LifetimeSeconds",
			errorTag:    "required",
		},
		{
			name: "Duplicated Prefix",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						Routes: []*RouteConfig{
							{
								Prefix:          "2001:db8::/64",
								LifetimeSeconds: 100,
							},
							{
								Prefix:          "2001:db8::/64",
								LifetimeSeconds: 100,
							},
						},
					},
				},
			},
			expectError: true,
			errorField:  "Routes",
			errorTag:    "unique",
		},

		// RDNSSConfig
		{
			name: "Valid RDNSSConfig",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						RDNSSes: []*RDNSSConfig{
							{
								LifetimeSeconds: 100,
								Addresses: []string{
									"fd00::1",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Multiple RDNSSConfig",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						RDNSSes: []*RDNSSConfig{
							{
								LifetimeSeconds: 100,
								Addresses: []string{
									"fd00::1",
								},
							},
							{
								LifetimeSeconds: 100,
								Addresses: []string{
									"fd00::2",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Nil RDNSSConfig",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						RDNSSes:                nil,
					},
				},
			},
			expectError: false,
		},
		{
			name: "Empty RDNSSConfig",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						RDNSSes:                []*RDNSSConfig{},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Nil RDNSSConfig Element",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						RDNSSes:                []*RDNSSConfig{nil},
					},
				},
			},
			expectError: true,
			errorField:  "LifetimeSeconds",
			errorTag:    "required",
		},
		{
			name: "No Addresses",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						RDNSSes: []*RDNSSConfig{
							{
								LifetimeSeconds: 100,
								Addresses:       []string{},
							},
						},
					},
				},
			},
			expectError: true,
			errorField:  "Addresses",
			errorTag:    "min",
		},
		{
			name: "Duplicated Addresses",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						RDNSSes: []*RDNSSConfig{
							{
								LifetimeSeconds: 100,
								Addresses: []string{
									"fd00::1",
									"fd00::1",
								},
							},
						},
					},
				},
			},
			expectError: true,
			errorField:  "Addresses",
			errorTag:    "unique",
		},
		{
			name: "Invalid Address",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						RDNSSes: []*RDNSSConfig{
							{
								LifetimeSeconds: 100,
								Addresses: []string{
									"10.0.0.1",
								},
							},
						},
					},
				},
			},
			expectError: true,
			errorField:  "Addresses[0]",
			errorTag:    "ipv6",
		},

		// DNSSLConfig
		{
			name: "Valid DNSSLConfig",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						DNSSLs: []*DNSSLConfig{
							{
								LifetimeSeconds: 100,
								DomainNames: []string{
									"example.com",
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Multiple DNSSLConfig",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						DNSSLs: []*DNSSLConfig{
							{
								LifetimeSeconds: 100,
								DomainNames: []string{
									"example.com",
									"foo.example.com",
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Nil DNSSLConfig",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						DNSSLs:                 nil,
					},
				},
			},
			expectError: false,
		},
		{
			name: "Empty DNSSLConfig",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						DNSSLs:                 []*DNSSLConfig{},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Nil DNSSLConfig Element",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						DNSSLs:                 []*DNSSLConfig{nil},
					},
				},
			},
			expectError: true,
			errorField:  "LifetimeSeconds",
			errorTag:    "required",
		},
		{
			name: "No DomainNames",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						DNSSLs: []*DNSSLConfig{
							{
								LifetimeSeconds: 100,
								DomainNames:     []string{},
							},
						},
					},
				},
			},
			expectError: true,
			errorField:  "DomainNames",
			errorTag:    "min",
		},
		{
			name: "Duplicated DomainNames",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						DNSSLs: []*DNSSLConfig{
							{
								LifetimeSeconds: 100,
								DomainNames: []string{
									"example.com",
									"example.com",
								},
							},
						},
					},
				},
			},
			expectError: true,
			errorField:  "DomainNames",
			errorTag:    "unique",
		},
		{
			name: "Qualified DomainName",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						DNSSLs: []*DNSSLConfig{
							{
								LifetimeSeconds: 100,
								DomainNames: []string{
									// Shouldn't be qualified.
									"example.com.",
								},
							},
						},
					},
				},
			},
			expectError: true,
			errorField:  "DomainNames[0]",
			errorTag:    "domain",
		},
		{
			name: "IP address DomainName",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						DNSSLs: []*DNSSLConfig{
							{
								LifetimeSeconds: 100,
								DomainNames: []string{
									"10.0.0.0",
								},
							},
						},
					},
				},
			},
			expectError: true,
			errorField:  "DomainNames[0]",
			errorTag:    "domain",
		},

		// NAT64PrefixConfig
		{
			name: "Nil NAT64PrefixConfig",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						NAT64Prefixes:          nil,
					},
				},
			},
			expectError: false,
		},
		{
			name: "Empty NAT64PrefixConfig",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						NAT64Prefixes:          []*NAT64PrefixConfig{},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Nil NAT64PrefixConfig Element",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						NAT64Prefixes:          []*NAT64PrefixConfig{nil},
					},
				},
			},
			expectError: true,
			errorField:  "Prefix",
			errorTag:    "required",
		},
		{
			name: "No NAT64Prefix",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						NAT64Prefixes: []*NAT64PrefixConfig{
							{
								LifetimeSeconds: ptr.To(1800),
							},
						},
					},
				},
			},
			expectError: true,
			errorField:  "Prefix",
			errorTag:    "required",
		},
		{
			name: "Multiple NAT64PrefixConfig",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						NAT64Prefixes: []*NAT64PrefixConfig{
							{
								Prefix:          "fc64:ff9b::/96",
								LifetimeSeconds: ptr.To(1800),
							},
							{
								Prefix:          "fd64:ff9b::/96",
								LifetimeSeconds: ptr.To(1800),
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Invalid NAT64Prefix length",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						NAT64Prefixes: []*NAT64PrefixConfig{
							{
								Prefix: "64:ff9b::/104",
							},
						},
					},
				},
			},
			expectError: true,
			errorField:  "Prefix",
			errorTag:    "invalid_prefix_len",
		},
		{
			name: "LifetimeSeconds = 65528",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						NAT64Prefixes: []*NAT64PrefixConfig{
							{
								Prefix:          "64:ff9b::/96",
								LifetimeSeconds: ptr.To(65528),
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "LifetimeSeconds < 0",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						NAT64Prefixes: []*NAT64PrefixConfig{
							{
								Prefix:          "64:ff9b::/96",
								LifetimeSeconds: ptr.To(-1),
							},
						},
					},
				},
			},
			expectError: true,
			errorField:  "LifetimeSeconds",
			errorTag:    "gte",
		},
		{
			name: "LifetimeSeconds > 65528",
			config: &Config{
				Interfaces: []*InterfaceConfig{
					{
						Name:                   "net0",
						RAIntervalMilliseconds: 1000,
						NAT64Prefixes: []*NAT64PrefixConfig{
							{
								Prefix:          "64:ff9b::/96",
								LifetimeSeconds: ptr.To(65529),
							},
						},
					},
				},
			},
			expectError: true,
			errorField:  "LifetimeSeconds",
			errorTag:    "lte",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.defaultAndValidate()
			if !tt.expectError {
				require.NoError(t, err)
				return
			}
			var verr validator.ValidationErrors
			require.ErrorAs(t, err, &verr)

			// Find the target error and we can ignore the rest.
			for _, v := range verr {
				if v.Field() == tt.errorField && v.Tag() == tt.errorTag {
					return
				}
			}

			require.Failf(t, "expected error not found", verr.Error())
		})
	}
}
