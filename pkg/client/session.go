package client

import (
	"fmt"
	"net"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/internal/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

const (
	highFrequencyStateRefreshPeriod = 10 * time.Second
	lowFrequencyStateRefreshPeriod  = 2 * time.Minute
)

// sender is an interface that defines the Send method for sending messages.
type sender interface {
	Send(dst *net.UDPAddr, msg *protocol.Message) error
}

// DeviceSession represents a session for a specific device.
type DeviceSession struct {
	sender  sender
	device  *Device
	inbound chan *protocol.Message // Map of sequence number to response channel
	seq     uint8                  // Sequence number for messages
	done    chan struct{}
}

// NewDeviceSession creates a new DeviceSession for the given device.
func NewDeviceSession(addr *net.UDPAddr, target [8]byte, sender sender) (*DeviceSession, error) {
	ds := &DeviceSession{
		sender:  sender,
		device:  NewDevice(addr, target),
		inbound: make(chan *protocol.Message, defaultRecvBufferSize),
		done:    make(chan struct{}),
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

// nextSeq increments the sequence number and returns the new value.
// It wraps around after reaching 255.
// nextSeq is not thread-safe and should be called with care in concurrent contexts.
func (s *DeviceSession) nextSeq() uint8 {
	s.seq++
	return s.seq
}

func (s *DeviceSession) run() {
	s.Send(DeviceStateMessages()...)
	hfTicker := time.NewTicker(highFrequencyStateRefreshPeriod)
	lfTicker := time.NewTicker(lowFrequencyStateRefreshPeriod)

	for {
		select {
		case <-s.done:
			return
		case <-hfTicker.C:
			s.Send(protocol.NewMessage(&packets.LightGet{}))
			hfTicker.Reset(highFrequencyStateRefreshPeriod)
		case <-lfTicker.C:
			s.Send(
				protocol.NewMessage(&packets.DeviceGetLabel{}),
				protocol.NewMessage(&packets.DeviceGetHostFirmware{}),
				protocol.NewMessage(&packets.DeviceGetLocation{}),
				protocol.NewMessage(&packets.DeviceGetGroup{}),
			)
			lfTicker.Reset(highFrequencyStateRefreshPeriod)
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

			switch p := msg.Payload.(type) {
			case *packets.DeviceStateLabel:
				s.device.Label = ParseLabel(p.Label)
			case *packets.LightState:
				s.device.Color = NewColor(p.Color)
				s.device.PoweredOn = p.Power > 0
			case *packets.DeviceStateVersion:
				s.device.SetProductID(p.Product)
			case *packets.DeviceStateHostFirmware:
				s.device.FirmwareVersion = fmt.Sprintf("%d.%d", p.VersionMajor, p.VersionMinor)
			case *packets.DeviceStateLocation:
				s.device.Location = ParseLabel(p.Label)
			case *packets.DeviceStateGroup:
				s.device.Group = ParseLabel(p.Label)
			case *packets.DeviceStateService, *packets.DeviceStateUnhandled: // Ignore these messages
			default:
				fmt.Println("Unhandled message type:", p.PayloadType())
			}
		case <-s.done:
			fmt.Println("Exiting recv loop for device:", s.device.Serial)
			return
		}
	}
}
