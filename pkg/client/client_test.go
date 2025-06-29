package client

import (
	"net"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/internal/protocol"
	"github.com/alessio-palumbo/lifxlan-go/internal/testutil"
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
		assert.Equal(t, recvMsg.Header.Target, target)
		assert.Equal(t, recvMsg.Header.Source, defaultSource)
		require.Equal(t, msg.Payload.PayloadType(), recvMsg.Payload.PayloadType())
	case <-time.After(time.Millisecond):
		t.Fatal("Expected data but got timeout")
	}

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
		assert.Equal(t, recvMsg.Header.Target, protocol.TargetBroadcast)
		assert.Equal(t, recvMsg.Header.IsTagged(), true)
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
		require.Equal(t, recvMsg.Header.Target, target)
	case <-time.After(time.Millisecond):
		t.Fatal("Did not receive message")
	}
}
