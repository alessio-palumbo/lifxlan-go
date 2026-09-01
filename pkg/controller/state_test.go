package controller

import (
	"errors"
	"math"
	"net"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func TestStateMessagesOrdersLightColorBeforePower(t *testing.T) {
	power := true
	hue, saturation, brightness := 210.0, 80.0, 60.0
	kelvin := uint16(3500)
	d := stateDevice(device.DeviceTypeLight)

	msgs, err := stateMessages(d, StateUpdate{Light: &LightStateUpdate{
		Power:      &power,
		Hue:        &hue,
		Saturation: &saturation,
		Brightness: &brightness,
		Kelvin:     &kelvin,
		Duration:   500 * time.Millisecond,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("messages = %d, want 2", len(msgs))
	}
	color, ok := msgs[0].Payload.(*packets.LightSetWaveformOptional)
	if !ok {
		t.Fatalf("first payload = %T, want *packets.LightSetWaveformOptional", msgs[0].Payload)
	}
	if !color.SetHue || !color.SetSaturation || !color.SetBrightness || !color.SetKelvin {
		t.Fatalf("color flags = %+v", color)
	}
	if color.Color.Kelvin != kelvin || color.Period != 500 {
		t.Fatalf("color = %+v", color)
	}
	powerMessage, ok := msgs[1].Payload.(*packets.LightSetPower)
	if !ok {
		t.Fatalf("second payload = %T, want *packets.LightSetPower", msgs[1].Payload)
	}
	if powerMessage.Level != math.MaxUint16 || powerMessage.Duration != 500 {
		t.Fatalf("power = %+v", powerMessage)
	}
}

func TestStateMessagesSupportsHybridLightAndRelays(t *testing.T) {
	power := false
	msgs, err := stateMessages(stateDevice(device.DeviceTypeHybrid), StateUpdate{
		Light: &LightStateUpdate{Power: &power},
		Relays: []RelayStateUpdate{
			{Index: 1, PoweredOn: true},
			{Index: 0, PoweredOn: false},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 {
		t.Fatalf("messages = %d, want 3", len(msgs))
	}
	if _, ok := msgs[0].Payload.(*packets.DeviceSetPower); !ok {
		t.Fatalf("first payload = %T, want *packets.DeviceSetPower", msgs[0].Payload)
	}
	firstRelay, ok := msgs[1].Payload.(*packets.RelaySetPower)
	if !ok || firstRelay.RelayIndex != 1 || firstRelay.Level != math.MaxUint16 {
		t.Fatalf("first relay = %#v", msgs[1].Payload)
	}
	secondRelay, ok := msgs[2].Payload.(*packets.RelaySetPower)
	if !ok || secondRelay.RelayIndex != 0 || secondRelay.Level != 0 {
		t.Fatalf("second relay = %#v", msgs[2].Payload)
	}
}

func TestStateMessagesRejectsUnsupportedCapabilities(t *testing.T) {
	power := true
	tests := []struct {
		name   string
		device device.Device
		update StateUpdate
	}{
		{
			name:   "light state on switch",
			device: stateDevice(device.DeviceTypeSwitch),
			update: StateUpdate{Light: &LightStateUpdate{Power: &power}},
		},
		{
			name:   "relay state on light",
			device: stateDevice(device.DeviceTypeLight),
			update: StateUpdate{Relays: []RelayStateUpdate{{Index: 0, PoweredOn: true}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := stateMessages(tt.device, tt.update)
			if !errors.Is(err, ErrUnsupportedCapability) {
				t.Fatalf("error = %v, want ErrUnsupportedCapability", err)
			}
		})
	}
}

func TestStateMessagesRejectsInvalidUpdates(t *testing.T) {
	nan := math.NaN()
	overBrightness := 101.0
	lowKelvin := uint16(1499)
	tests := []StateUpdate{
		{},
		{Light: &LightStateUpdate{}},
		{Light: &LightStateUpdate{Hue: &nan}},
		{Light: &LightStateUpdate{Brightness: &overBrightness}},
		{Light: &LightStateUpdate{Kelvin: &lowKelvin}},
		{Light: &LightStateUpdate{Duration: -time.Millisecond}},
		{Relays: []RelayStateUpdate{{Index: -1}}},
		{Relays: []RelayStateUpdate{{Index: 256}}},
		{Relays: []RelayStateUpdate{{Index: 0}, {Index: 0}}},
	}
	for i, update := range tests {
		if _, err := stateMessages(stateDevice(device.DeviceTypeHybrid), update); !errors.Is(err, ErrInvalidState) {
			t.Errorf("update %d error = %v, want ErrInvalidState", i, err)
		}
	}
}

func TestControllerSetStateReportsPartialSend(t *testing.T) {
	serial := stateSerial()
	sender := &failingStateSender{failAt: 2}
	d := stateDevice(device.DeviceTypeLight)
	d.Serial = serial
	ctrl := controllerWithStateSender(sender, d)
	power := true
	hue := 120.0

	err := ctrl.SetState(serial, StateUpdate{Light: &LightStateUpdate{Power: &power, Hue: &hue}})
	var sendErr *StateSendError
	if !errors.As(err, &sendErr) {
		t.Fatalf("error = %v, want *StateSendError", err)
	}
	if sendErr.Sent != 1 || sendErr.Total != 2 {
		t.Fatalf("send error = %+v", sendErr)
	}
	if len(sender.attempts) != 2 {
		t.Fatalf("attempts = %d, want 2", len(sender.attempts))
	}
	for _, msg := range sender.attempts {
		if msg.Target() != [8]byte(serial) {
			t.Fatalf("target = %x, want %x", msg.Target(), serial)
		}
	}
}

func TestControllerSetStateReturnsDeviceNotFound(t *testing.T) {
	ctrl := &Controller{sessions: make(map[device.Serial]*deviceSession)}
	power := true
	err := ctrl.SetState(stateSerial(), StateUpdate{Light: &LightStateUpdate{Power: &power}})
	if !errors.Is(err, ErrDeviceNotFound) {
		t.Fatalf("error = %v, want ErrDeviceNotFound", err)
	}
}

func stateDevice(deviceType device.DeviceType) device.Device {
	return device.Device{
		Address: &net.UDPAddr{IP: net.IPv4(192, 168, 0, 2)},
		Serial:  stateSerial(),
		Type:    deviceType,
		ColorProperties: device.ColorProperties{
			TemperatureRange: device.TemperatureRange{Min: 1500, Max: 9000},
		},
	}
}

func stateSerial() device.Serial {
	return device.Serial([8]byte{0xd0, 0x73, 0xd5, 0x01, 0x02, 0x03})
}

type failingStateSender struct {
	failAt   int
	attempts []*protocol.Message
}

func (s *failingStateSender) Send(_ *net.UDPAddr, msg *protocol.Message) error {
	s.attempts = append(s.attempts, msg)
	if len(s.attempts) == s.failAt {
		return errors.New("send failed")
	}
	return nil
}

func controllerWithStateSender(sender sender, devices ...device.Device) *Controller {
	ctrl := &Controller{sessions: make(map[device.Serial]*deviceSession)}
	for i := range devices {
		d := devices[i]
		ctrl.sessions[d.Serial] = &deviceSession{sender: sender, device: &d}
	}
	return ctrl
}
