package testutil

import (
	"net"
	"testing"

	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/stretchr/testify/require"
)

func NewMockUDPServer(t *testing.T, handler func(*protocol.Message, *net.UDPAddr)) (*net.UDPConn, *net.UDPAddr) {
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	conn, err := net.ListenUDP("udp", addr)
	require.NoError(t, err)

	go func() {
		buf := make([]byte, 1024)
		for {
			n, src, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}

			var msg protocol.Message
			if err := msg.UnmarshalBinary(buf[:n]); err != nil {
				// skip malformed
				continue
			}
			handler(&msg, src)
		}
	}()

	return conn, conn.LocalAddr().(*net.UDPAddr)
}
