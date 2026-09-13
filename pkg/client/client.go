package client

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
)

// ErrNoBroadcastInterface indicates that automatic selection found no
// broadcast-capable IPv4 interface.
var ErrNoBroadcastInterface = errors.New("no suitable broadcast interface found")

// BroadcastInterfaceNotFoundError indicates that a configured interface name
// or index did not match a broadcast-capable IPv4 interface.
type BroadcastInterfaceNotFoundError struct {
	Name  string
	Index int
}

func (e *BroadcastInterfaceNotFoundError) Error() string {
	switch {
	case e.Name != "":
		return fmt.Sprintf("broadcast interface %q not found", e.Name)
	case e.Index > 0:
		return fmt.Sprintf("broadcast interface index %d not found", e.Index)
	default:
		return "configured broadcast interface not found"
	}
}

const (
	// lifxPort is the port LIFX devices listen to for broadcast messages.
	lifxPort = 56700

	recvBufferSize        = 1024
	defaultSource  uint32 = 0x00000002

	broadcastUpIface = net.FlagUp | net.FlagBroadcast
)

// Client is a UDP client that can be used to send and receive LIFX messages on the LAN.
type Client struct {
	conn               *net.UDPConn
	source             uint32
	broadcastAddr      *net.UDPAddr
	broadcastInterface *BroadcastInterface
}

// Config contains optional user-configurable fields.
type Config struct {
	// Source is the unique identifier set by the client and returned
	// by devices in all responses.
	// Source must be greater than 1 or some devices on older firmware
	// might either ignore (0) or broadcast the response (1).
	Source uint32

	// BroadcastAddr overrides automatic broadcast interface selection.
	// If Port is 0, the default LIFX UDP port is used.
	BroadcastAddr *net.UDPAddr
	// BroadcastInterfaceName selects a broadcast-capable IPv4 interface by name.
	// If it has multiple suitable addresses, the first candidate is used.
	BroadcastInterfaceName string
	// BroadcastInterfaceIndex selects a broadcast-capable IPv4 interface by index.
	BroadcastInterfaceIndex int
}

// HandlerFunc processes a received message and address.
type HandlerFunc func(*protocol.Message, *net.UDPAddr)

// BroadcastInterface describes one interface usable for subnet broadcast.
type BroadcastInterface struct {
	Index     int
	Name      string
	IP        net.IP
	Broadcast net.IP
	Flags     net.Flags
}

// NewClient returns an instance of Client with an initialised UDP connection.
func NewClient(cfg *Config) (*Client, error) {
	source := defaultSource
	if cfg != nil {
		if cfg.Source != 0 {
			if cfg.Source < defaultSource {
				return nil, fmt.Errorf("source must be greater than 1")
			}
			source = cfg.Source
		}
	}

	bAddr, resolvedInterface, err := resolveBroadcastTarget(lifxPort, cfg)
	if err != nil {
		return nil, err
	}

	addr := &net.UDPAddr{Port: 0, IP: net.IPv4zero}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}

	return &Client{
		conn:               conn,
		source:             source,
		broadcastAddr:      bAddr,
		broadcastInterface: resolvedInterface,
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

// BroadcastInterface returns the OS interface selected for broadcast. The
// boolean is false when Config.BroadcastAddr supplied an exact address. The
// returned value is independent and may be safely modified by the caller.
func (c *Client) BroadcastInterface() (BroadcastInterface, bool) {
	if c.broadcastInterface == nil {
		return BroadcastInterface{}, false
	}
	return cloneBroadcastInterface(*c.broadcastInterface), true
}

// BroadcastInterfaces returns broadcast-capable IPv4 interfaces that can be
// used for LIFX discovery.
func BroadcastInterfaces() ([]BroadcastInterface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("could not list interfaces: %w", err)
	}
	return broadcastInterfacesFrom(ifaces, func(iface net.Interface) ([]net.Addr, error) {
		return iface.Addrs()
	}), nil
}

// resolveBroadcastUDPAddress computes and returns the UDP broadcast address for
// cfg. Without an override it preserves the historical behavior: first suitable
// network interface wins.
func resolveBroadcastUDPAddress(port int, cfg *Config) (*net.UDPAddr, error) {
	addr, _, err := resolveBroadcastTarget(port, cfg)
	return addr, err
}

func resolveBroadcastTarget(port int, cfg *Config) (*net.UDPAddr, *BroadcastInterface, error) {
	if cfg != nil && cfg.BroadcastAddr != nil {
		return broadcastUDPAddr(cfg.BroadcastAddr.IP, cfg.BroadcastAddr.Port, port), nil, nil
	}

	candidates, err := BroadcastInterfaces()
	if err != nil {
		return nil, nil, err
	}
	return resolveBroadcastTargetFromCandidates(port, cfg, candidates)
}

func resolveBroadcastUDPAddressFromCandidates(port int, cfg *Config, candidates []BroadcastInterface) (*net.UDPAddr, error) {
	addr, _, err := resolveBroadcastTargetFromCandidates(port, cfg, candidates)
	return addr, err
}

func resolveBroadcastTargetFromCandidates(port int, cfg *Config, candidates []BroadcastInterface) (*net.UDPAddr, *BroadcastInterface, error) {
	if cfg != nil {
		if cfg.BroadcastInterfaceName != "" {
			for _, candidate := range candidates {
				if candidate.Name == cfg.BroadcastInterfaceName {
					resolved := cloneBroadcastInterface(candidate)
					return broadcastUDPAddr(candidate.Broadcast, port, port), &resolved, nil
				}
			}
			return nil, nil, &BroadcastInterfaceNotFoundError{Name: cfg.BroadcastInterfaceName}
		}
		if cfg.BroadcastInterfaceIndex > 0 {
			for _, candidate := range candidates {
				if candidate.Index == cfg.BroadcastInterfaceIndex {
					resolved := cloneBroadcastInterface(candidate)
					return broadcastUDPAddr(candidate.Broadcast, port, port), &resolved, nil
				}
			}
			return nil, nil, &BroadcastInterfaceNotFoundError{Index: cfg.BroadcastInterfaceIndex}
		}
	}

	if len(candidates) == 0 {
		return nil, nil, ErrNoBroadcastInterface
	}
	resolved := cloneBroadcastInterface(candidates[0])
	return broadcastUDPAddr(candidates[0].Broadcast, port, port), &resolved, nil
}

func broadcastInterfacesFrom(ifaces []net.Interface, addrs func(net.Interface) ([]net.Addr, error)) []BroadcastInterface {
	var out []BroadcastInterface
	for _, iface := range ifaces {
		if iface.Flags&broadcastUpIface != broadcastUpIface {
			continue
		}

		ifaceAddrs, err := addrs(iface)
		if err != nil {
			// skip bad interface
			continue
		}

		for _, addr := range ifaceAddrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok || ipnet.IP.To4() == nil || len(ipnet.Mask) != net.IPv4len {
				continue
			}

			ip := ipnet.IP.To4()
			broadcast := make(net.IP, net.IPv4len)
			for i := range net.IPv4len {
				broadcast[i] = ip[i] | ^ipnet.Mask[i]
			}

			out = append(out, BroadcastInterface{
				Index:     iface.Index,
				Name:      iface.Name,
				IP:        append(net.IP(nil), ip...),
				Broadcast: broadcast,
				Flags:     iface.Flags,
			})
		}
	}
	return out
}

func broadcastUDPAddr(ip net.IP, port, defaultPort int) *net.UDPAddr {
	if port == 0 {
		port = defaultPort
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		ip = ipv4
	}
	return &net.UDPAddr{IP: append(net.IP(nil), ip...), Port: port}
}

func cloneBroadcastInterface(iface BroadcastInterface) BroadcastInterface {
	iface.IP = append(net.IP(nil), iface.IP...)
	iface.Broadcast = append(net.IP(nil), iface.Broadcast...)
	return iface
}
