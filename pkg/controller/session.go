package controller

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	log "github.com/sirupsen/logrus"
)

const (
	defaultRecvBufferSize  = 10
	preflightHandshakeWait = time.Second
)

// sender is an interface that defines message sending.
type sender interface {
	Send(dst *net.UDPAddr, msg *protocol.Message) error
}

// DeviceSession represents a session for a specific device.
type DeviceSession struct {
	sender  sender
	inbound chan *protocol.Message
	seq     atomic.Uint32
	done    chan struct{}
	cfg     *Config

	// mu protects read/write access of DeviceState
	mu     sync.RWMutex
	device *device.Device
}

// NewDeviceSession creates a new DeviceSession for the given device.
// It spins up a goroutine to periodically query devices for state updates and
// a second one to parse devices messages and update Device state.
func NewDeviceSession(addr *net.UDPAddr, serial device.Serial, sender sender, cfg *Config) (*DeviceSession, error) {
	ds := &DeviceSession{
		sender:  sender,
		device:  device.NewDevice(addr, serial),
		inbound: make(chan *protocol.Message, defaultRecvBufferSize),
		done:    make(chan struct{}),
		cfg:     cfg,
	}

	go ds.recvloop()
	go ds.run()

	return ds, nil
}

// Close closes the DeviceSession, stopping the recv loop and cleaning up resources.
func (s *DeviceSession) Close() error {
	close(s.done)
	return nil
}

// Send sends one or more messages to the device.
func (s *DeviceSession) Send(msgs ...*protocol.Message) error {
	for _, msg := range msgs {
		msg.SetTarget(s.device.Serial)
		msg.SetSequence(s.nextSeq())
		if err := s.sender.Send(s.device.Address, msg); err != nil {
			return fmt.Errorf("failed to send message to device %s: %v", s.device.Serial, err)
		}
	}
	return nil
}

// DeviceSnapshot returns a copy of a Device with its current device state.
func (s *DeviceSession) DeviceSnapshot() device.Device {
	s.mu.Lock()
	defer s.mu.Unlock()
	return *s.device
}

// nextSeq increments the sequence number and returns the new value.
// It wraps around after reaching 255.
func (s *DeviceSession) nextSeq() uint8 {
	return uint8(s.seq.Add(1))
}

// run performs a short-lived pre-flight handshake to gather required device state
// after which it periodically queries the device for state updates.
// It uses a ticker for high frequency state changes and one for low frequency ones.
func (s *DeviceSession) run() {
	required := requiredStateMessages()
	for len(required) > 0 {
		s.Send(required...)
		s.waitForOrTimeout(&required, preflightHandshakeWait)
	}

	hfTicker := time.NewTicker(s.cfg.highFrequencyStateRefreshPeriod)
	lfTicker := time.NewTicker(s.cfg.lowFrequencyStateRefreshPeriod)

	for {
		select {
		case <-s.done:
			return
		case <-hfTicker.C:
			s.Send(s.device.HighFreqStateMessages()...)
			hfTicker.Reset(s.cfg.highFrequencyStateRefreshPeriod)
		case <-lfTicker.C:
			s.Send(s.device.LowFreqStateMessages()...)
			lfTicker.Reset(s.cfg.lowFrequencyStateRefreshPeriod)
		}
	}
}

// recvloop listens for incoming messages from the device and processes them.
func (s *DeviceSession) recvloop() {
	for {
		select {
		case msg := <-s.inbound:
			if msg == nil {
				continue
			}

			s.mu.Lock()
			switch p := msg.Payload.(type) {
			case *packets.DeviceStateLabel:
				s.device.Label = device.ParseLabel(p.Label)
			case *packets.LightState:
				s.device.Color = device.NewColor(p.Color)
				s.device.PoweredOn = p.Power > 0
			case *packets.DeviceStateVersion:
				s.device.SetProductInfo(p.Product)
			case *packets.DeviceStateHostFirmware:
				s.device.FirmwareVersion = fmt.Sprintf("%d.%d", p.VersionMajor, p.VersionMinor)
			case *packets.DeviceStateLocation:
				s.device.Location = device.ParseLabel(p.Label)
			case *packets.DeviceStateGroup:
				s.device.Group = device.ParseLabel(p.Label)
			case *packets.TileStateDeviceChain:
				s.device.SetMatrixProperties(p)
			case *packets.TileState64:
				s.device.SetMatrixState(p)
			case *packets.DeviceStatePower:
				s.device.PoweredOn = p.Level > 0
			case *packets.MultiZoneExtendedStateMultiZone:
				// TODO
			case *packets.DeviceStateService, *packets.DeviceStateUnhandled: // Ignore these messages
			default:
				log.WithField("serial", s.device.Serial).
					WithField("payload", msg.Payload.PayloadType()).
					Debug("Session: Unhandled message type")
			}
			s.device.LastSeenAt = time.Now()
			s.mu.Unlock()
		case <-s.done:
			log.WithField("serial", s.device.Serial).Info("Exiting device recv loop")
			return
		}
	}
}

// waitForOrTimeout polls the list of messages at set intervals and check if they
// have been fulfilled at which point it returns. The list is updated at each iteration
// and if timeout occurs it returns early.
func (s *DeviceSession) waitForOrTimeout(required *[]*protocol.Message, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for {
		var retryMsgs []*protocol.Message
		for _, m := range *required {
			s.mu.RLock()
			if f := messageDoneFuncs[m.Payload]; f != nil && !f(s.device) {
				retryMsgs = append(retryMsgs, m)
			}
			s.mu.RUnlock()
		}
		*required = retryMsgs

		if len(retryMsgs) == 0 {
			return
		}
		if time.Now().After(deadline) {
			// timed out with some messgaes not retried.
			return
		}

		// polling delay
		time.Sleep(50 * time.Millisecond)
	}
}

// requiredStateMessages returns a list of protocol messages to gather critical information
// about the state of a Device.
func requiredStateMessages() []*protocol.Message {
	return []*protocol.Message{
		protocol.NewMessage(&packets.DeviceGetLabel{}),
		protocol.NewMessage(&packets.DeviceGetVersion{}),
		protocol.NewMessage(&packets.LightGet{}),
		protocol.NewMessage(&packets.DeviceGetHostFirmware{}),
		protocol.NewMessage(&packets.DeviceGetLocation{}),
		protocol.NewMessage(&packets.DeviceGetGroup{}),
		protocol.NewMessage(&packets.TileGetDeviceChain{}),
	}
}

// messageDoneFuncs maps a message to a function to checks whether the message has been fulfilled.
var messageDoneFuncs = map[packets.Payload]func(*device.Device) bool{
	&packets.DeviceGetLabel{}:        func(d *device.Device) bool { return d.Label != "" },
	&packets.DeviceGetVersion{}:      func(d *device.Device) bool { return d.ProductID > 0 },
	&packets.DeviceGetHostFirmware{}: func(d *device.Device) bool { return d.FirmwareVersion != "" },
	&packets.DeviceGetLocation{}:     func(d *device.Device) bool { return d.Location != "" },
	&packets.DeviceGetGroup{}:        func(d *device.Device) bool { return d.Group != "" },
	&packets.TileGetDeviceChain{}: func(d *device.Device) bool {
		return d.LightType != device.LightTypeMatrix || d.MatrixProperties.ChainLength > 0
	},
}
