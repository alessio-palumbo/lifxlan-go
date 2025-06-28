package client

import (
	"net"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/internal/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	"github.com/stretchr/testify/require"
)

func TestClient_Send(t *testing.T) {
	serverAddr := &net.UDPAddr{IP: net.IPv4zero, Port: 0}
	serverConn, err := net.ListenUDP("udp4", serverAddr)
	require.NoError(t, err)
	defer serverConn.Close()

	// Get actual port assigned
	serverPort := serverConn.LocalAddr().(*net.UDPAddr).Port

	client, err := NewClient(nil)
	require.NoError(t, err)
	defer client.Close()

	payload := &packets.DeviceGetService{}
	msg := protocol.NewMessage(payload)
	msg.SetTarget([8]byte{0, 0, 0, 0, 0, 0, 0, 1})

	dst := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: serverPort}

	err = client.Send(dst, msg)
	require.NoError(t, err)

	buf := make([]byte, 1024)
	serverConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, addr, err := serverConn.ReadFromUDP(buf)
	require.NoError(t, err)
	require.NotNil(t, addr)
	require.Greater(t, n, 0)

	// Try to unmarshal the received message
	var received protocol.Message
	err = received.UnmarshalBinary(buf[:n])
	require.NoError(t, err)
	require.Equal(t, msg.Payload.PayloadType(), received.Payload.PayloadType())
}

func TestClient_SendBroadcast(t *testing.T) {
	client, err := NewClient(nil)
	require.NoError(t, err)
	defer client.Close()

	payload := &packets.DeviceGetService{}
	msg := protocol.NewMessage(payload)

	err = client.SendBroadcast(msg)
	require.NoError(t, err)
}
