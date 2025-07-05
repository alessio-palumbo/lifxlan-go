package client

import (
	"fmt"
	"net"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
)

const (
	// lifxPort is the port LIFX devices listen to for broadcast messages.
	lifxPort = 56700

	recvBufferSize        = 1024
	defaultSource  uint32 = 0x00000002

	broadcastUpIface = net.FlagUp | net.FlagBroadcast
)

// Client is a UDP client that can be used to send and receive LIFX messages on the LAN.
type Client struct {
	conn          *net.UDPConn
	source        uint32
	broadcastAddr *net.UDPAddr
}

// Config contains optional user-configurable fields.
type Config struct {
	// Source is the unique identifier set by the client and returned
	// by devices in all responses.
	// Source must be greater than 1 or some devices on older firmware
	// might either ignore (0) or broadcast the response (1).
	Source uint32
}

// HandlerFunc processes a received message and address.
type HandlerFunc func(*protocol.Message, *net.UDPAddr)

// NewClient returns an instance of Client with an initialised UDP connection.
func NewClient(cfg *Config) (*Client, error) {
	addr := &net.UDPAddr{Port: 0, IP: net.IPv4zero}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	bAddr, err := resolveBroadcastUDPAddress(lifxPort)
	if err != nil {
		return nil, err
	}

	source := defaultSource
	if cfg != nil {
		if cfg.Source != 0 {
			if cfg.Source < defaultSource {
				return nil, fmt.Errorf("source must be greater than 1")
			}
			source = cfg.Source
		}
	}

	return &Client{
		conn:          conn,
		source:        source,
		broadcastAddr: bAddr,
	}, nil
}

// Close closes the Client underlying UDP connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// Send sends a message to the specified destination address.
func (c *Client) Send(dst *net.UDPAddr, msg *protocol.Message) error {
	msg.SetSource(c.source)

	data, err := msg.MarshalBinary()
	if err != nil {
		return err
	}

	_, err = c.conn.WriteToUDP(data, dst)
	return err
}

// SendBroadcast sends a LIFX protocol message to the broadcast address.
func (c *Client) SendBroadcast(msg *protocol.Message) error {
	msg.SetTarget(protocol.TargetBroadcast)
	return c.Send(c.broadcastAddr, msg)
}

// Receive listens for incoming UDP packets and decodes them into LIFX protocol messages.
// It reads from the underlying connection until the specified timeout expires or a single
// message is received (if recvOne is true). For each successfully decoded message,
// the provided handler function is invoked with the message and sender's address.
// Malformed messages are silently ignored.
func (c *Client) Receive(timeout time.Duration, recvOne bool, handler HandlerFunc) error {
	if timeout > 0 {
		c.conn.SetReadDeadline(time.Now().Add(timeout))
		// Reset deadline after reading
		defer c.conn.SetReadDeadline(time.Time{})
	}

	buf := make([]byte, recvBufferSize)

	for {
		n, addr, err := c.conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			return err
		}

		var msg protocol.Message
		if err := msg.UnmarshalBinary(buf[:n]); err != nil {
			// skip malformed
			continue
		}

		handler(&msg, addr)
		if recvOne {
			break
		}
	}

	return nil
}

// SetConnDeadline sets the connection deadline.
func (c *Client) SetConnDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

// resolveBroadcastUDPAddress computes and returns the subnet-specific UDP
// broadcast address for the first suitable network interface.
// It uses the interface's IPv4 address and netmask to calculate the address.
func resolveBroadcastUDPAddress(port int) (*net.UDPAddr, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("could not list interfaces: %w", err)
	}

	for _, iface := range ifaces {
		if iface.Flags&broadcastUpIface != broadcastUpIface {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			// skip bad interface
			continue
		}

		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok || ipnet.IP.To4() == nil {
				continue
			}

			ip := ipnet.IP.To4()
			mask := ipnet.Mask
			broadcast := make(net.IP, 4)
			for i := range 4 {
				broadcast[i] = ip[i] | ^mask[i]
			}

			return &net.UDPAddr{
				IP:   broadcast,
				Port: port,
			}, nil
		}
	}

	return nil, fmt.Errorf("no suitable broadcast interface found")
}
