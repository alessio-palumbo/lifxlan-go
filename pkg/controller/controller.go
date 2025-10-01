package controller

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/internal/logutil"
	"github.com/alessio-palumbo/lifxlan-go/pkg/client"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/enums"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	log "github.com/sirupsen/logrus"
)

const (
	defaultDiscoveryPeriod                 = 500 * time.Millisecond
	defaultHighFrequencyStateRefreshPeriod = 10 * time.Second
	defaultLowFrequencyStateRefreshPeriod  = 2 * time.Minute

	preflightHandshakeTimeout = 5 * time.Second
	preflightHandshakeWait    = time.Second
	minLivenessTimeout        = 30 * time.Second
	livenessTimeoutMultiplier = 5
)

// Controller manages discovery and message routing for multiple
// devices on the LAN.
type Controller struct {
	client   Client
	recvDone chan struct{}
	cfg      *Config

	closeOnce sync.Once
	mu        sync.RWMutex
	sessions  map[device.Serial]*DeviceSession
}

type Client interface {
	Send(dst *net.UDPAddr, msg *protocol.Message) error
	SendBroadcast(msg *protocol.Message) error
	Receive(timeout time.Duration, recvOne bool, handler client.HandlerFunc) error
	SetConnDeadline(t time.Time) error
	Close() error
}

// Config contains configurable options for discovery and state updates.
type Config struct {
	// Configurable
	discoveryPeriod                 time.Duration
	highFrequencyStateRefreshPeriod time.Duration
	lowFrequencyStateRefreshPeriod  time.Duration

	// Non configurable
	deviceLivenessTimeout     time.Duration
	preflightHandshakeTimeout time.Duration
	preflightHandshakeWait    time.Duration
}

// setLivenessTimeout sets the inactivity period after which a device is considered
// offline and its session marked for termination.
//
// The timeout is derived from the shorter of the configured high- and low-frequency
// refresh periods, multiplied by a user-defined multiplier to tolerate jitter or
// occasional packet loss. This ensures that as long as either probe type is running,
// the device is expected to respond within this window.
//
// A minimum threshold is enforced to avoid overly aggressive checks when very low
// refresh periods are configured (e.g. 1s). No maximum is applied, since the probe
// intervals themselves define the heartbeat expectation.
func (c *Config) setLivenessTimeout() {
	t := min(c.highFrequencyStateRefreshPeriod, c.lowFrequencyStateRefreshPeriod) * time.Duration(livenessTimeoutMultiplier)
	if t > minLivenessTimeout {
		c.deviceLivenessTimeout = t
		return
	}
	c.deviceLivenessTimeout = minLivenessTimeout
}

// New returns a Controller that periodically discovers LIFX devices
// on the LAN and creates individual sessions for message routing.
func New(opts ...Option) (*Controller, error) {
	logutil.Init()

	ctrl := &Controller{
		recvDone: make(chan struct{}),
		sessions: make(map[device.Serial]*DeviceSession),
		cfg: &Config{
			discoveryPeriod:                 defaultDiscoveryPeriod,
			highFrequencyStateRefreshPeriod: defaultHighFrequencyStateRefreshPeriod,
			lowFrequencyStateRefreshPeriod:  defaultLowFrequencyStateRefreshPeriod,
			preflightHandshakeTimeout:       preflightHandshakeTimeout,
			preflightHandshakeWait:          preflightHandshakeWait,
		},
	}
	for _, opt := range opts {
		if err := opt(ctrl); err != nil {
			return nil, err
		}
	}
	// Set liveness timeout after any option has been applied.
	ctrl.cfg.setLivenessTimeout()

	if ctrl.client == nil {
		c, err := client.NewClient(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create client: %w", err)
		}
		ctrl.client = c
	}

	go ctrl.recvloop()

	// Perform an intial discovery and exit early, if needed.
	if err := ctrl.Discover(); err != nil {
		return nil, fmt.Errorf("failed to discover devices: %w", err)
	}
	go ctrl.periodicDiscovery()

	return ctrl, nil
}

// Close closes the Controller, stopping the recv loop and terminating all device sessions.
// Close is idempotent.
func (c *Controller) Close() error {
	// Close the client connection and wait for the recv loop to finish.
	c.closeOnce.Do(func() {
		c.client.SetConnDeadline(time.Now())
		<-c.recvDone
		c.client.Close()

		for serial, session := range c.sessions {
			if err := session.Close(); err != nil {
				log.WithError(err).WithField("serial", serial).Error("Failed to close device session")
			}
		}
		clear(c.sessions)
		log.Info("Controller closed")
	})

	return nil
}

// Discover broadcasts a LIFX discover packet.
func (c *Controller) Discover() error {
	msg := protocol.NewMessage(&packets.DeviceGetService{})
	return c.client.SendBroadcast(msg)
}

// periodicDiscovery periodically looks for new devices on the network.
func (c *Controller) periodicDiscovery() {
	ticker := time.NewTicker(c.cfg.discoveryPeriod)

	for {
		select {
		case <-c.recvDone:
			return
		case <-ticker.C:
			_ = c.Discover()
			ticker.Reset(c.cfg.discoveryPeriod)
		}
	}
}

// addSession adds a new device session.
func (c *Controller) addSession(addr *net.UDPAddr, serial device.Serial) error {
	cb := func(serial device.Serial) { c.terminateSession(serial) }
	session, err := NewDeviceSession(addr, serial, c.client, c.cfg, cb)
	if err != nil {
		return fmt.Errorf("failed to create device session: %w", err)
	}

	c.mu.Lock()
	c.sessions[serial] = session
	c.mu.Unlock()

	return nil
}

// terminateSession terminates a device session.
func (c *Controller) terminateSession(serial device.Serial) {
	c.mu.Lock()
	if session, ok := c.sessions[serial]; ok {
		session.Close()
		delete(c.sessions, serial)
	}
	c.mu.Unlock()
}

// Send sends the given message to the given UDP address, if a session exists.
func (c *Controller) Send(serial device.Serial, msg *protocol.Message) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if s, ok := c.sessions[serial]; ok {
		return s.Send(msg)
	}
	return nil
}

// GetDevices returns the list of devices that have a session.
func (c *Controller) GetDevices() []device.Device {
	var devices []device.Device
	c.mu.RLock()
	for _, session := range c.sessions {
		devices = append(devices, session.DeviceSnapshot())
	}
	c.mu.RUnlock()

	device.SortDevices(devices)
	return devices
}

// recv listens for incoming messages from devices and dispatches them to the appropriate session.
func (c *Controller) recvloop() {
	defer close(c.recvDone)

	if err := c.client.Receive(0, false, func(msg *protocol.Message, addr *net.UDPAddr) {
		serial := device.Serial(msg.Target())

		c.mu.RLock()
		session, hasSession := c.sessions[serial]
		c.mu.RUnlock()

		if state, ok := msg.Payload.(*packets.DeviceStateService); ok {
			if !hasSession && state.Service == enums.DeviceServiceDEVICESERVICEUDP {
				if err := c.addSession(addr, serial); err != nil {
					log.WithError(err).WithField("serial", serial).Error("Failed to spin device worker")
				}
			}
		} else if hasSession {
			select {
			case session.inbound <- msg:
			default:
				// If the channel is full, we skip the message to avoid blocking.
				log.WithField("serial", serial).
					WithField("payload", msg.Payload.PayloadType()).
					Warning("Channel full, skipping message")
			}
		}
	}); err != nil {
		// If Receive exits due to an error make sure the Controller shuts down gracefully.
		c.Close()
	}
}
