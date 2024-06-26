// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of go-ra

package ra

import (
	"context"
	"net"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/mdlayher/ndp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

// We use a common parameter for most of the Eventually.
func eventully(t *testing.T, f func() bool) {
	require.Eventually(t, f, time.Second*1, time.Millisecond*10)
}

func assertRAInterval(ct *assert.CollectT, sock *fakeSock, interval time.Duration) bool {
	// wait until we get 3 RAs
	timeout, cancel := context.WithTimeout(context.Background(), time.Second*1)

	ras := []fakeRA{}
outer:
	for {
		select {
		case <-timeout.Done():
			cancel()
			return assert.Fail(ct, "couldn't get 3 RAs in time")
		case ra := <-sock.txMulticastCh():
			ras = append(ras, ra)
			if len(ras) == 3 {
				cancel()
				break outer
			}
		}
	}

	// Ensure the interval is correct. We let 60ms of error margin.
	mergin := float64(60 * time.Millisecond)
	diff0 := ras[1].tstamp.Sub(ras[0].tstamp)
	diff1 := ras[2].tstamp.Sub(ras[1].tstamp)

	return assert.InDelta(ct, interval, diff0, mergin) && assert.InDelta(ct, interval, diff1, mergin)
}

func TestDaemonHappyPath(t *testing.T) {
	config := &Config{
		Interfaces: []*InterfaceConfig{
			{
				Name:                       "net0",
				RAIntervalMilliseconds:     100,
				CurrentHopLimit:            10,
				Managed:                    true,
				Other:                      true,
				Preference:                 "high",
				RouterLifetimeSeconds:      10,
				ReachableTimeMilliseconds:  10000,
				RetransmitTimeMilliseconds: 10000,
				MTU:                        1500,
				Prefixes: []*PrefixConfig{
					{
						Prefix:                   "fd00::/64",
						OnLink:                   true,
						Autonomous:               true,
						PreferredLifetimeSeconds: ptr.To(100),
						ValidLifetimeSeconds:     ptr.To(200),
					},
				},
				Routes: []*RouteConfig{
					{
						Prefix:          "2001:db8::/64",
						Preference:      "low",
						LifetimeSeconds: 100,
					},
					{
						Prefix:          "2001:db8:1::/64",
						Preference:      "high",
						LifetimeSeconds: 200,
					},
				},
				RDNSSes: []*RDNSSConfig{
					{
						LifetimeSeconds: 300,
						Addresses:       []string{"2001:db8::1", "2001:db8::2"},
					},
				},
				DNSSLs: []*DNSSLConfig{
					{
						LifetimeSeconds: 400,
						DomainNames:     []string{"example.com", "foo.example.com"},
					},
				},
				NAT64Prefixes: []*NAT64PrefixConfig{
					{
						Prefix:          "64:ff9b::/96",
						LifetimeSeconds: ptr.To(1800),
					},
				},
			},
			{
				Name:                   "net1",
				RAIntervalMilliseconds: 100,
			},
		},
	}

	reg := newFakeSockRegistry()

	// Create a fake device watcher and inject an initial device state
	devWatcher := newFakeDeviceWatcher("net0", "net1")
	devWatcher.update("net0", deviceState{isUp: true, addr: net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77}})
	devWatcher.update("net1", deviceState{isUp: true, addr: net.HardwareAddr{0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}})

	d, err := NewDaemon(
		config,
		withSocketConstructor(reg.newSock),
		withDeviceWatcher(devWatcher),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	go d.Run(ctx)
	t.Cleanup(func() {
		if t.Failed() {
			cancel()
		}
	})

	t.Run("Ensure socket is created", func(t *testing.T) {
		eventully(t, func() bool {
			_, err0 := reg.getSock("net0")
			_, err1 := reg.getSock("net1")
			return assert.NoError(t, err0) && assert.NoError(t, err1)
		})
	})

	t.Run("Ensure unsolicited RA is sent with the specified interval", func(t *testing.T) {
		var sock *fakeSock

		sock, err = reg.getSock("net0")
		require.NoError(t, err)
		require.EventuallyWithT(t, func(ct *assert.CollectT) {
			assertRAInterval(ct, sock, time.Millisecond*100)
		}, time.Second*1, time.Millisecond*100)

		sock, err = reg.getSock("net1")
		require.NoError(t, err)
		require.EventuallyWithT(t, func(ct *assert.CollectT) {
			assertRAInterval(ct, sock, time.Millisecond*100)
		}, time.Second*1, time.Millisecond*100)
	})

	t.Run("Ensure the RA parameter is reflected to the packet", func(t *testing.T) {
		var sock *fakeSock

		sock, err = reg.getSock("net0")
		require.NoError(t, err)

		// Sampling one RA
		ra := <-sock.txMulticastCh()

		// Check the parameters
		require.Equal(t, uint8(10), ra.msg.CurrentHopLimit)
		require.True(t, ra.msg.ManagedConfiguration)
		require.True(t, ra.msg.OtherConfiguration)
		require.Equal(t, ndp.High, ra.msg.RouterSelectionPreference)
		require.Equal(t, time.Second*10, ra.msg.RouterLifetime)
		require.Equal(t, time.Millisecond*10000, ra.msg.ReachableTime)
		require.Equal(t, time.Millisecond*10000, ra.msg.RetransmitTimer)

		// Find MTU option
		var mtuOption *ndp.MTU
		for _, option := range ra.msg.Options {
			if opt, ok := option.(*ndp.MTU); ok {
				mtuOption = opt
				break
			}
		}
		require.NotNil(t, mtuOption, "MTU option is not advertised")
		require.Equal(t, uint32(1500), mtuOption.MTU, "Invalid MTU")

		// Find and check Source Link-Layer Address option
		var slaOption *ndp.LinkLayerAddress
		for _, option := range ra.msg.Options {
			if opt, ok := option.(*ndp.LinkLayerAddress); ok {
				slaOption = opt
				break
			}
		}
		require.NotNil(t, slaOption, "Source Link-Layer Address option is not advertised")
		require.Equal(t, net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77}, slaOption.Addr)

		// Find and check Prefix Information options
		prefixOptions := map[netip.Addr]*ndp.PrefixInformation{}
		for _, option := range ra.msg.Options {
			if opt, ok := option.(*ndp.PrefixInformation); ok {
				prefixOptions[opt.Prefix] = opt
			}
		}
		prefix := netip.MustParseAddr("fd00::")
		require.Contains(t, prefixOptions, prefix)
		prefixInfo := prefixOptions[prefix]
		require.Equal(t, uint8(64), prefixInfo.PrefixLength)
		require.True(t, prefixInfo.OnLink)
		require.True(t, prefixInfo.AutonomousAddressConfiguration)
		require.Equal(t, time.Second*100, prefixInfo.PreferredLifetime)
		require.Equal(t, time.Second*200, prefixInfo.ValidLifetime)

		// Find and check Route Information options
		routeOptions := map[netip.Addr]*ndp.RouteInformation{}
		for _, option := range ra.msg.Options {
			if opt, ok := option.(*ndp.RouteInformation); ok {
				routeOptions[opt.Prefix] = opt
			}
		}
		prefix0 := netip.MustParseAddr("2001:db8::")
		prefix1 := netip.MustParseAddr("2001:db8:1::")
		require.Contains(t, routeOptions, prefix0)
		require.Contains(t, routeOptions, prefix1)
		route0 := routeOptions[prefix0]
		route1 := routeOptions[prefix1]
		require.Equal(t, uint8(64), route0.PrefixLength)
		require.Equal(t, uint8(64), route1.PrefixLength)
		require.Equal(t, ndp.Low, route0.Preference)
		require.Equal(t, ndp.High, route1.Preference)
		require.Equal(t, time.Second*100, route0.RouteLifetime)
		require.Equal(t, time.Second*200, route1.RouteLifetime)

		// Find and check RDNSS options
		var rdnssOptions *ndp.RecursiveDNSServer
		for _, option := range ra.msg.Options {
			if opt, ok := option.(*ndp.RecursiveDNSServer); ok {
				rdnssOptions = opt
				break
			}
		}
		require.NotNil(t, rdnssOptions, "RDNSS option is not advertised")
		require.Equal(t, time.Second*300, rdnssOptions.Lifetime)
		require.ElementsMatch(t, []netip.Addr{netip.MustParseAddr("2001:db8::1"), netip.MustParseAddr("2001:db8::2")}, rdnssOptions.Servers)

		// Find and check DNSSL options
		var dnsslOptions *ndp.DNSSearchList
		for _, option := range ra.msg.Options {
			if opt, ok := option.(*ndp.DNSSearchList); ok {
				dnsslOptions = opt
				break
			}
		}
		require.NotNil(t, dnsslOptions, "DNSSL option is not advertised")
		require.Equal(t, time.Second*400, dnsslOptions.Lifetime)
		require.ElementsMatch(t, []string{"example.com", "foo.example.com"}, dnsslOptions.DomainNames)

		// Find and check NAT64Prefix options
		nat64prefixOptions := map[netip.Prefix]*ndp.PREF64{}
		for _, option := range ra.msg.Options {
			if opt, ok := option.(*ndp.PREF64); ok {
				nat64prefixOptions[opt.Prefix] = opt
			}
		}
		nat64prefix := netip.MustParsePrefix("64:ff9b::/96")
		require.Contains(t, nat64prefixOptions, nat64prefix)
		nat64prefixInfo := nat64prefixOptions[nat64prefix]
		require.Equal(t, int(96), nat64prefixInfo.Prefix.Bits())
		require.Equal(t, time.Second*1800, nat64prefixInfo.Lifetime)
	})

	t.Run("Ensure the status is running and the result is ordered by name", func(t *testing.T) {
		status := d.Status()
		require.NoError(t, err)
		require.Len(t, status.Interfaces, 2)
		assert.Equal(t, "net0", status.Interfaces[0].Name)
		assert.Equal(t, "net1", status.Interfaces[1].Name)
		assert.Equal(t, Running, status.Interfaces[0].State)
		assert.Equal(t, Running, status.Interfaces[1].State)
	})

	t.Run("Ensure Source Link Layer Address option is updated after device MAC address change", func(t *testing.T) {
		// Update the MAC address of net0
		devWatcher.update("net0", deviceState{isUp: true, addr: net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x78}})

		sock, err := reg.getSock("net0")
		require.NoError(t, err)

		eventully(t, func() bool {
			// Sampling one RA
			ra := <-sock.txMulticastCh()

			// Find and check Source Link-Layer Address option
			var slaOption *ndp.LinkLayerAddress
			for _, option := range ra.msg.Options {
				if opt, ok := option.(*ndp.LinkLayerAddress); ok {
					slaOption = opt
					break
				}
			}

			require.NotNil(t, slaOption, "Source Link-Layer Address option is not advertised")

			return slices.Equal(net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x78}, slaOption.Addr)
		})
	})

	t.Run("Ensure unsolicited RA interval is updated after reload", func(t *testing.T) {
		// Update the interval of net1. net0 should remain the same.
		config.Interfaces[1].RAIntervalMilliseconds = 200

		// Reload
		timeout, cancelTimeout := context.WithTimeout(context.Background(), time.Second*1)
		err := d.Reload(timeout, config)
		require.NoError(t, err)
		cancelTimeout()

		require.EventuallyWithT(t, func(ct *assert.CollectT) {
			sock0, err := reg.getSock("net0")
			if !assert.NoError(t, err) {
				return
			}
			sock1, err := reg.getSock("net1")
			if !assert.NoError(t, err) {
				return
			}
			assertRAInterval(ct, sock0, time.Millisecond*100)
			assertRAInterval(ct, sock1, time.Millisecond*200)
		}, time.Second*1, time.Millisecond*100)
	})

	t.Run("Ensure RS is replied with unicast RA", func(t *testing.T) {
		sock, err := reg.getSock("net0")
		require.NoError(t, err)

		from := netip.MustParseAddr("fe80::1%net0")

		rs := &ndp.RouterSolicitation{}

		// Send RS
		sock.rxCh() <- fakeRS{msg: rs, from: from}

		// Wait for solicited RA
		timeout, cancelTimeout := context.WithTimeout(context.Background(), time.Second*1)
		select {
		case ra := <-sock.txLLUnicastCh():
			require.Equal(t, ra.to, from)
		case <-timeout.Done():
			require.Fail(t, "timeout waiting for RA")
		}
		cancelTimeout()
	})

	t.Run("Ensure unsolicited RA is stopped after removing configuration", func(t *testing.T) {
		// Remove net1
		config.Interfaces = config.Interfaces[:1]

		// Reload
		timeout, cancelTimeout := context.WithTimeout(context.Background(), time.Second*1)
		err := d.Reload(timeout, config)
		require.NoError(t, err)
		cancelTimeout()

		require.EventuallyWithT(t, func(ct *assert.CollectT) {
			sock0, err := reg.getSock("net0")
			if !assert.NoError(t, err) {
				return
			}
			sock1, err := reg.getSock("net1")
			if !assert.NoError(t, err) {
				return
			}
			assertRAInterval(ct, sock0, time.Millisecond*100)
			assert.True(ct, sock1.isClosed())
		}, time.Second*1, time.Millisecond*100)
	})

	t.Run("Ensure unsolicited RA is stopped after stopping the daemon", func(t *testing.T) {
		// Cancel the daemon's context
		cancel()
		eventully(t, func() bool {
			sock0, err := reg.getSock("net0")
			if !assert.NoError(t, err) {
				return false
			}
			sock1, err := reg.getSock("net1")
			if !assert.NoError(t, err) {
				return false
			}
			return assert.True(t, sock0.isClosed()) && assert.True(t, sock1.isClosed())
		})
	})
}
