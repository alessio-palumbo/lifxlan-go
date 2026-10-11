package client

import (
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExplicitBroadcastRefreshStaysOnSelectedInterface(t *testing.T) {
	old := BroadcastInterface{Name: "wifi0", Index: 1, IP: net.IPv4(172, 16, 0, 2), Broadcast: net.IPv4(172, 16, 0, 255)}
	c := &Client{broadcastAddr: &net.UDPAddr{IP: old.Broadcast, Port: 56700}, broadcastInterface: &old}
	newIface := BroadcastInterface{Name: "wifi0", Index: 1, IP: net.IPv4(192, 168, 1, 42), Broadcast: net.IPv4(192, 168, 1, 255)}
	other := BroadcastInterface{Name: "other0", Index: 2, IP: net.IPv4(10, 0, 0, 2), Broadcast: net.IPv4(10, 0, 0, 255)}
	require.NoError(t, c.refreshBroadcastFromCandidates([]BroadcastInterface{other, newIface}))
	got, ok := c.BroadcastInterface()
	require.True(t, ok)
	require.Equal(t, "wifi0", got.Name)
	require.True(t, got.IP.Equal(newIface.IP))
	got.IP[0] = 9
	fresh, _ := c.BroadcastInterface()
	require.True(t, fresh.IP.Equal(newIface.IP))
	require.Error(t, c.refreshBroadcastFromCandidates([]BroadcastInterface{other}))
	fresh, _ = c.BroadcastInterface()
	require.Equal(t, "wifi0", fresh.Name)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 100 {
			c.BroadcastInterface()
		}
	}()
	for range 100 {
		require.NoError(t, c.refreshBroadcastFromCandidates([]BroadcastInterface{newIface}))
	}
	wg.Wait()
}

func TestBroadcastRefreshHonorsPinnedAndFixedConfiguration(t *testing.T) {
	iface := BroadcastInterface{Name: "wifi0", Index: 4, IP: net.IPv4(192, 168, 1, 42), Broadcast: net.IPv4(192, 168, 1, 255)}
	c := &Client{broadcastConfig: Config{BroadcastInterfaceIndex: 4}}
	require.NoError(t, c.refreshBroadcastFromCandidates([]BroadcastInterface{iface}))
	got, _ := c.BroadcastInterface()
	require.Equal(t, 4, got.Index)
	fixed := &net.UDPAddr{IP: net.IPv4(192, 168, 1, 255), Port: 1234}
	c = &Client{broadcastConfig: copyConfig(&Config{BroadcastAddr: fixed}), broadcastAddr: cloneUDPAddress(fixed)}
	fixed.IP[0] = 10
	require.NoError(t, c.refreshBroadcastFromCandidates(nil))
	require.True(t, c.broadcastAddr.IP.Equal(net.IPv4(192, 168, 1, 255)))
	require.Equal(t, 1234, c.broadcastAddr.Port)
}
