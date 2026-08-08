package client

import (
	"net"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/internal/testutil"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/enums"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_SendUnicast(t *testing.T) {
	recvCh := make(chan *protocol.Message, 1)
	conn, saddr := testutil.NewMockUDPServer(t, func(msg *protocol.Message, _ *net.UDPAddr) {
		recvCh <- msg
	})
	defer conn.Close()

	client, err := NewClient(nil)
	require.NoError(t, err)
	defer client.Close()

	payload := &packets.LightGet{}
	msg := protocol.NewMessage(payload)
	target := [8]byte{0, 0, 0, 0, 0, 0, 0, 1}
	msg.SetTarget(target)

	err = client.Send(saddr, msg)
	require.NoError(t, err)

	select {
	case recvMsg := <-recvCh:
		assert.Equal(t, recvMsg, msg)
		assert.Equal(t, recvMsg.Target(), target)
		assert.Equal(t, recvMsg.Source(), defaultSource)
		require.Equal(t, msg.Payload.PayloadType(), recvMsg.Payload.PayloadType())
	case <-time.After(time.Millisecond):
		t.Fatal("Expected data but got timeout")
	}

}

func TestNewClientUsesExplicitBroadcastAddress(t *testing.T) {
	broadcastAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9}
	client, err := NewClient(&Config{BroadcastAddr: broadcastAddr})
	require.NoError(t, err)
	defer client.Close()

	broadcastAddr.IP[0] = 10
	assert.Equal(t, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1).To4(), Port: 9}, client.broadcastAddr)
}

func TestNewClientDefaultsExplicitBroadcastAddressPort(t *testing.T) {
	client, err := NewClient(&Config{BroadcastAddr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)}})
	require.NoError(t, err)
	defer client.Close()

	assert.Equal(t, lifxPort, client.broadcastAddr.Port)
}

func TestClient_SendBroadcast(t *testing.T) {
	recvCh := make(chan *protocol.Message, 1)
	conn, saddr := testutil.NewMockUDPServer(t, func(msg *protocol.Message, _ *net.UDPAddr) {
		recvCh <- msg
	})
	defer conn.Close()

	client, err := NewClient(nil)
	// Manually set broadcast address to mock server
	client.broadcastAddr = saddr
	require.NoError(t, err)
	defer client.Close()

	payload := &packets.DeviceGetService{}
	msg := protocol.NewMessage(payload)

	err = client.SendBroadcast(msg)
	require.NoError(t, err)

	select {
	case recvMsg := <-recvCh:
		assert.Equal(t, recvMsg, msg)
		assert.Equal(t, recvMsg.Target(), protocol.TargetBroadcast)
		require.Equal(t, msg.Payload.PayloadType(), recvMsg.Payload.PayloadType())
	case <-time.After(time.Millisecond):
		t.Fatal("Expected data but got timeout")
	}

}

func TestClient_Receive(t *testing.T) {
	// Explicityly bind address to loopback for testing.
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	conn, err := net.ListenUDP("udp", addr)
	require.NoError(t, err)
	c := &Client{conn: conn}
	defer c.Close()

	recvCh := make(chan *protocol.Message, 1)
	go func() {
		err := c.Receive(time.Second, true, func(msg *protocol.Message, addr *net.UDPAddr) {
			recvCh <- msg
		})
		require.NoError(t, err)
	}()

	// Give Receive a moment to start listening
	time.Sleep(time.Millisecond)

	payload := &packets.DeviceStateService{
		Service: enums.DeviceServiceDEVICESERVICEUDP,
		Port:    lifxPort,
	}
	msg := protocol.NewMessage(payload)
	target := [8]byte{0, 0, 0, 0, 0, 0, 0, 1}
	msg.SetTarget(target)

	data, err := msg.MarshalBinary() // assuming you have a protocol.Encode
	require.NoError(t, err)

	// Write to the client's own listening address
	_, err = c.conn.WriteToUDP(data, c.conn.LocalAddr().(*net.UDPAddr))
	require.NoError(t, err)

	select {
	case recvMsg := <-recvCh:
		require.Equal(t, recvMsg.Target(), target)
	case <-time.After(time.Millisecond):
		t.Fatal("Did not receive message")
	}
}

func TestBroadcastInterfacesFromFiltersAndCalculatesBroadcast(t *testing.T) {
	ifaces := []net.Interface{
		{Index: 1, Name: "down0", Flags: net.FlagBroadcast},
		{Index: 2, Name: "nobroadcast0", Flags: net.FlagUp},
		{Index: 3, Name: "bad0", Flags: broadcastUpIface},
		{Index: 4, Name: "wifi0", Flags: broadcastUpIface},
	}
	addrsByName := map[string][]net.Addr{
		"bad0": {
			mustCIDR(t, "2001:db8::1/64"),
		},
		"wifi0": {
			mustCIDR(t, "192.168.1.42/24"),
			mustCIDR(t, "10.0.0.5/8"),
		},
	}

	got := broadcastInterfacesFrom(ifaces, func(iface net.Interface) ([]net.Addr, error) {
		return addrsByName[iface.Name], nil
	})

	require.Len(t, got, 2)
	assert.Equal(t, 4, got[0].Index)
	assert.Equal(t, "wifi0", got[0].Name)
	assert.Equal(t, net.IPv4(192, 168, 1, 42).To4(), got[0].IP)
	assert.Equal(t, net.IPv4(192, 168, 1, 255).To4(), got[0].Broadcast)
	assert.Equal(t, net.IPv4(10, 255, 255, 255).To4(), got[1].Broadcast)
}

func TestBroadcastInterfacesFromSkipsInterfacesWithAddressError(t *testing.T) {
	ifaces := []net.Interface{
		{Index: 1, Name: "bad0", Flags: broadcastUpIface},
		{Index: 2, Name: "wifi0", Flags: broadcastUpIface},
	}

	got := broadcastInterfacesFrom(ifaces, func(iface net.Interface) ([]net.Addr, error) {
		if iface.Name == "bad0" {
			return nil, assert.AnError
		}
		return []net.Addr{mustCIDR(t, "192.168.1.42/24")}, nil
	})

	require.Len(t, got, 1)
	assert.Equal(t, "wifi0", got[0].Name)
}

func TestResolveBroadcastUDPAddressFromCandidates(t *testing.T) {
	candidates := []BroadcastInterface{
		{Index: 10, Name: "eth0", Broadcast: net.IPv4(10, 0, 0, 255)},
		{Index: 20, Name: "wifi0", Broadcast: net.IPv4(192, 168, 1, 255)},
	}

	tests := map[string]struct {
		cfg  *Config
		want *net.UDPAddr
	}{
		"default first candidate": {
			want: &net.UDPAddr{IP: net.IPv4(10, 0, 0, 255).To4(), Port: lifxPort},
		},
		"name": {
			cfg:  &Config{BroadcastInterfaceName: "wifi0"},
			want: &net.UDPAddr{IP: net.IPv4(192, 168, 1, 255).To4(), Port: lifxPort},
		},
		"index": {
			cfg:  &Config{BroadcastInterfaceIndex: 20},
			want: &net.UDPAddr{IP: net.IPv4(192, 168, 1, 255).To4(), Port: lifxPort},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := resolveBroadcastUDPAddressFromCandidates(lifxPort, tc.cfg, candidates)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestResolveBroadcastUDPAddressFromCandidatesRejectsMissingSelection(t *testing.T) {
	candidates := []BroadcastInterface{{Index: 10, Name: "eth0", Broadcast: net.IPv4(10, 0, 0, 255)}}

	_, err := resolveBroadcastUDPAddressFromCandidates(lifxPort, &Config{BroadcastInterfaceName: "wifi0"}, candidates)
	require.Error(t, err)

	_, err = resolveBroadcastUDPAddressFromCandidates(lifxPort, &Config{BroadcastInterfaceIndex: 20}, candidates)
	require.Error(t, err)
}

func TestResolveBroadcastUDPAddressFromCandidatesRejectsEmptyDefault(t *testing.T) {
	_, err := resolveBroadcastUDPAddressFromCandidates(lifxPort, nil, nil)
	require.Error(t, err)
}

func TestBroadcastUDPAddrUsesDefaultPortAndCopiesIP(t *testing.T) {
	ip := net.IPv4(192, 168, 1, 255).To4()
	got := broadcastUDPAddr(ip, 0, lifxPort)
	ip[0] = 10

	assert.Equal(t, &net.UDPAddr{IP: net.IPv4(192, 168, 1, 255).To4(), Port: lifxPort}, got)
}

func mustCIDR(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	ip, ipnet, err := net.ParseCIDR(cidr)
	require.NoError(t, err)
	ipnet.IP = ip
	return ipnet
}
