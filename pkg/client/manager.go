package client

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/internal/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/enums"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

const (
	defaultRecvBufferSize  = 10
	defaultDiscoveryPeriod = 500 * time.Millisecond
)

type DeviceManager struct {
	client   *Client
	recvDone chan struct{}

	mu       sync.RWMutex
	sessions map[string]*DeviceSession
}

// NewDeviceManager creates a new DeviceManager that starts listening for devices.
func NewDeviceManager() (*DeviceManager, error) {
	client, err := NewClient(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	dm := &DeviceManager{
		client:   client,
		recvDone: make(chan struct{}),
		sessions: make(map[string]*DeviceSession),
	}
	go dm.recvloop()

	// Perform an intial discovery and exit early, if needed.
	if err := dm.Discover(); err != nil {
		return nil, fmt.Errorf("failed to discover devices: %w", err)
	}
	go dm.periodicDiscovery(defaultDiscoveryPeriod)

	return dm, nil
}

// Close closes the DeviceManager, stopping the recv loop and closing all device sessions.
func (d *DeviceManager) Close() error {
	// Close the client connection and wait for the recv loop to finish.
	d.client.conn.SetDeadline(time.Now())
	<-d.recvDone
	d.client.Close()

	for _, session := range d.sessions {
		if err := session.Close(); err != nil {
			return fmt.Errorf("failed to close device session: %w", err)
		}
	}
	clear(d.sessions)
	fmt.Println("Device manager closed")
	return nil
}

func (d *DeviceManager) Discover() error {
	msg := protocol.NewMessage(&packets.DeviceGetService{})
	return d.client.SendBroadcast(msg)
}

// periodicDiscovery periodically looks for new devices on the network.
func (d *DeviceManager) periodicDiscovery(period time.Duration) {
	ticker := time.NewTicker(period)

	for {
		select {
		case <-d.recvDone:
			return
		case <-ticker.C:
			_ = d.Discover()
			ticker.Reset(period)
		}
	}
}

func (d *DeviceManager) addSession(addr *net.UDPAddr, target [8]byte) error {
	session, err := NewDeviceSession(addr, target, d.client)
	if err != nil {
		return fmt.Errorf("failed to create device session: %w", err)
	}

	d.mu.Lock()
	d.sessions[addr.IP.String()] = session
	d.mu.Unlock()

	return nil
}

func (d *DeviceManager) Send(addr *net.UDPAddr, msg *protocol.Message) error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if s, ok := d.sessions[addr.IP.String()]; ok {
		return s.Send(msg)
	}
	return nil
}

func (d *DeviceManager) GetDevices() []Device {
	var devices []Device
	d.mu.RLock()
	for _, session := range d.sessions {
		devices = append(devices, *session.device)
	}
	d.mu.RUnlock()

	SortDevices(devices)
	return devices
}

// recv listens for incoming messages from devices and dispatches them to the appropriate session.
func (d *DeviceManager) recvloop() {
	defer close(d.recvDone)
	buf := make([]byte, recvBufferSize)

	for {
		n, addr, err := d.client.conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			fmt.Println("failed to read from UDP, terminating session:", err)
			d.Close()
			return
		}

		var msg protocol.Message
		if err := msg.UnmarshalBinary(buf[:n]); err != nil {
			// skip malformed
			continue
		}

		d.mu.RLock()
		session, hasSession := d.sessions[addr.IP.String()]
		d.mu.RUnlock()

		if state, ok := msg.Payload.(*packets.DeviceStateService); ok {
			if !hasSession && state.Service == enums.DeviceServiceDEVICESERVICEUDP {
				if err := d.addSession(addr, msg.Header.Target); err != nil {
					fmt.Println("Failed to spin device worker:", err)
				}
			}
		} else if hasSession {
			select {
			case session.inbound <- &msg:
			default:
				// If the channel is full, we skip the message to avoid blocking.
				fmt.Println("Channel full, skipping message for", Serial(msg.Header.Target).String(), msg.Payload.PayloadType())
			}
		}
	}
}
