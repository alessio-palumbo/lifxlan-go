package controller

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/internal/protocol"
	"github.com/alessio-palumbo/lifxlan-go/pkg/client"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/enums"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

const (
	defaultRecvBufferSize  = 10
	defaultDiscoveryPeriod = 500 * time.Millisecond
)

// Controller manages discovery and message routing for multiple
// devices on the LAN.
type Controller struct {
	client   *client.Client
	recvDone chan struct{}

	mu       sync.RWMutex
	sessions map[string]*DeviceSession
}

// New returns a Controller that periodically discovers LIFX devices
// on the LAN and creates individual sessions for message routing.
// It currently does not check whether
func New() (*Controller, error) {
	client, err := client.NewClient(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	dm := &Controller{
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

// Close closes the Controller, stopping the recv loop and closing all device sessions.
func (d *Controller) Close() error {
	// Close the client connection and wait for the recv loop to finish.
	d.client.SetConnDeadline(time.Now())
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

// Discover broadcasts a LIFX discover packet.
func (d *Controller) Discover() error {
	msg := protocol.NewMessage(&packets.DeviceGetService{})
	return d.client.SendBroadcast(msg)
}

// periodicDiscovery periodically looks for new devices on the network.
func (d *Controller) periodicDiscovery(period time.Duration) {
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

// addSession adds a new device session.
func (d *Controller) addSession(addr *net.UDPAddr, target [8]byte) error {
	session, err := NewDeviceSession(addr, target, d.client)
	if err != nil {
		return fmt.Errorf("failed to create device session: %w", err)
	}

	d.mu.Lock()
	d.sessions[addr.IP.String()] = session
	d.mu.Unlock()

	return nil
}

// Send sends the given message to the given UDP address, if a session exists.
func (d *Controller) Send(addr *net.UDPAddr, msg *protocol.Message) error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if s, ok := d.sessions[addr.IP.String()]; ok {
		return s.Send(msg)
	}
	return nil
}

// GetDevices returns the list of devices that have a session.
func (d *Controller) GetDevices() []Device {
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
func (d *Controller) recvloop() {
	defer close(d.recvDone)
	defer d.Close()

	d.client.Receive(0, false, func(msg *protocol.Message, addr *net.UDPAddr) {
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
			case session.inbound <- msg:
			default:
				// If the channel is full, we skip the message to avoid blocking.
				fmt.Println("Channel full, skipping message for", Serial(msg.Header.Target).String(), msg.Payload.PayloadType())
			}
		}
	})
}
