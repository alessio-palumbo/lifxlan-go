package controller

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/internal/logutil"
	"github.com/alessio-palumbo/lifxlan-go/internal/protocol"
	"github.com/alessio-palumbo/lifxlan-go/pkg/client"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/enums"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	log "github.com/sirupsen/logrus"
)

const (
	defaultDiscoveryPeriod                 = 500 * time.Millisecond
	defaulthighFrequencyStateRefreshPeriod = 10 * time.Second
	defaultlowFrequencyStateRefreshPeriod  = 2 * time.Minute
)

// Controller manages discovery and message routing for multiple
// devices on the LAN.
type Controller struct {
	client   *client.Client
	recvDone chan struct{}
	cfg      *Config

	mu       sync.RWMutex
	sessions map[string]*DeviceSession
}

// Config contains configurable options for discovery and state updates.
type Config struct {
	discoveryPeriod                 time.Duration
	highFrequencyStateRefreshPeriod time.Duration
	lowFrequencyStateRefreshPeriod  time.Duration
}

// New returns a Controller that periodically discovers LIFX devices
// on the LAN and creates individual sessions for message routing.
func New(cfg *Config) (*Controller, error) {
	logutil.Init()

	client, err := client.NewClient(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	dm := &Controller{
		client:   client,
		recvDone: make(chan struct{}),
		sessions: make(map[string]*DeviceSession),
		cfg:      parseConfig(cfg),
	}

	go dm.recvloop()

	// Perform an intial discovery and exit early, if needed.
	if err := dm.Discover(); err != nil {
		return nil, fmt.Errorf("failed to discover devices: %w", err)
	}
	go dm.periodicDiscovery()

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

	log.Info("Device manager closed")
	return nil
}

// Discover broadcasts a LIFX discover packet.
func (d *Controller) Discover() error {
	msg := protocol.NewMessage(&packets.DeviceGetService{})
	return d.client.SendBroadcast(msg)
}

// periodicDiscovery periodically looks for new devices on the network.
func (d *Controller) periodicDiscovery() {
	ticker := time.NewTicker(d.cfg.discoveryPeriod)

	for {
		select {
		case <-d.recvDone:
			return
		case <-ticker.C:
			_ = d.Discover()
			ticker.Reset(d.cfg.discoveryPeriod)
		}
	}
}

// addSession adds a new device session.
func (d *Controller) addSession(addr *net.UDPAddr, target [8]byte) error {
	session, err := NewDeviceSession(addr, target, d.client, d.cfg)
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
					log.WithError(err).WithField("serial", Serial(msg.Header.Target)).Error("Failed to spin device worker")
				}
			}
		} else if hasSession {
			select {
			case session.inbound <- msg:
			default:
				// If the channel is full, we skip the message to avoid blocking.
				log.WithField("serial", Serial(msg.Header.Target)).
					WithField("payload", msg.Payload.PayloadType()).
					Warning("Channel full, skipping message")
			}
		}
	})
}

// parseConfig returns a Config with any missing property set to its default.
func parseConfig(cfg *Config) *Config {
	if cfg == nil {
		cfg = new(Config)
	}
	if cfg.discoveryPeriod == 0 {
		cfg.discoveryPeriod = defaultDiscoveryPeriod
	}
	if cfg.highFrequencyStateRefreshPeriod == 0 {
		cfg.highFrequencyStateRefreshPeriod = defaulthighFrequencyStateRefreshPeriod
	}
	if cfg.lowFrequencyStateRefreshPeriod == 0 {
		cfg.lowFrequencyStateRefreshPeriod = defaultlowFrequencyStateRefreshPeriod
	}
	return cfg
}
