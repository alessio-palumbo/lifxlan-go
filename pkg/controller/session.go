package controller

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	log "github.com/sirupsen/logrus"
)

const (
	defaultRecvBufferSize = 10
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
	device *Device
}

// NewDeviceSession creates a new DeviceSession for the given device.
// It spins up a goroutine to periodically query devices for state updates and
// a second one to parse devices messages and update Device state.
func NewDeviceSession(addr *net.UDPAddr, serial Serial, sender sender, cfg *Config) (*DeviceSession, error) {
	ds := &DeviceSession{
		sender:  sender,
		device:  NewDevice(addr, serial),
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

func (s *DeviceSession) DeviceSnapshot() Device {
	s.mu.Lock()
	defer s.mu.Unlock()
	return *s.device
}

// nextSeq increments the sequence number and returns the new value.
// It wraps around after reaching 255.
func (s *DeviceSession) nextSeq() uint8 {
	return uint8(s.seq.Add(1))
}

// run periodically query the device for state updates.
// It uses a ticker for high frequency state changes and one for low frequency ones.
func (s *DeviceSession) run() {
	s.Send(DeviceStateMessages()...)
	hfTicker := time.NewTicker(s.cfg.highFrequencyStateRefreshPeriod)
	lfTicker := time.NewTicker(s.cfg.lowFrequencyStateRefreshPeriod)

	for {
		select {
		case <-s.done:
			return
		case <-hfTicker.C:
			s.Send(protocol.NewMessage(&packets.LightGet{}))
			hfTicker.Reset(s.cfg.highFrequencyStateRefreshPeriod)
		case <-lfTicker.C:
			s.Send(
				protocol.NewMessage(&packets.DeviceGetLabel{}),
				protocol.NewMessage(&packets.DeviceGetHostFirmware{}),
				protocol.NewMessage(&packets.DeviceGetLocation{}),
				protocol.NewMessage(&packets.DeviceGetGroup{}),
			)
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
				s.device.Label = ParseLabel(p.Label)
			case *packets.LightState:
				s.device.Color = NewColor(p.Color)
				s.device.PoweredOn = p.Power > 0
			case *packets.DeviceStateVersion:
				s.device.SetProductInfo(p.Product)
			case *packets.DeviceStateHostFirmware:
				s.device.FirmwareVersion = fmt.Sprintf("%d.%d", p.VersionMajor, p.VersionMinor)
			case *packets.DeviceStateLocation:
				s.device.Location = ParseLabel(p.Label)
			case *packets.DeviceStateGroup:
				s.device.Group = ParseLabel(p.Label)
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
