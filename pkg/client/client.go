package client

import (
	"fmt"
	"net"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/internal/protocol"
)

const (
	lifxPort       = 56700
	recvBufferSize = 1024

	defaultSource   uint32 = 0x00000002
	defaultDeadline        = 2 * time.Second
)

type Client struct {
	conn          *net.UDPConn
	source        uint32
	deadline      time.Duration
	broadcastAddr *net.UDPAddr
}

type Config struct {
	// Source is the unique identifier set by the client and returned
	// by devices in all responses.
	// Source must be greater than 1 or some devices on older firmware
	// might either ignore (0) or broadcast the response (1).
	Source   uint32
	Deadline time.Duration
}

// HandlerFunc processes a received message and address.
type HandlerFunc func(*protocol.Message, *net.UDPAddr)

// NewClient creates a new LIFX client with the specified configuration.
// If cfg is nil, default values will be used for source and deadline.
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
	deadline := defaultDeadline
	if cfg != nil {
		if cfg.Source != 0 {
			if cfg.Source < defaultSource {
				return nil, fmt.Errorf("source must be greater than 1")
			}
			source = cfg.Source
		}
		if cfg.Deadline > 0 {
			deadline = cfg.Deadline
		}
	}

	fmt.Printf("Started client with source '%s' and deadline '%d'\n", sourceString(source), deadline)

	return &Client{
		conn:          conn,
		source:        source,
		deadline:      deadline,
		broadcastAddr: bAddr,
	}, nil
}

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

// SendBroadcast sends a message to the broadcast address for LIFX devices.
func (c *Client) SendBroadcast(msg *protocol.Message) error {
	msg.SetTarget(protocol.TargetBroadcast)
	return c.Send(c.broadcastAddr, msg)
}

// Receive reads UDP messages until the deadline is hit or recvOne is true and a valid message has been received.
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
			return nil
		}
	}

	return nil
}

func (c *Client) SetConnDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func resolveBroadcastUDPAddress(port int) (*net.UDPAddr, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("could not list interfaces: %w", err)
	}

	for _, iface := range ifaces {
		// Skip interfaces that are down or loopback
		if iface.Flags&(net.FlagUp|net.FlagBroadcast) != (net.FlagUp | net.FlagBroadcast) {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue // skip bad interface
		}

		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok || ipnet.IP.To4() == nil {
				continue // skip non-IPv4 or invalid
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

func sourceString(s uint32) string {
	b := []byte{
		byte(s & 0xFF),
		byte((s >> 8) & 0xFF),
		byte((s >> 16) & 0xFF),
		byte((s >> 24) & 0xFF),
	}

	return string(b)
}
