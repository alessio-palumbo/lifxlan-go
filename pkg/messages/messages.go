package messages

import (
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/enums"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

// SetPowerOn sets a device power to its maximum value of 65535.
// An optional time.Duration argument can be specified to apply a custom transition.
func SetPowerOn(d ...time.Duration) *protocol.Message {
	if len(d) > 0 {
		return protocol.NewMessage(&packets.LightSetPower{Level: math.MaxUint16, Duration: uint32(d[0].Milliseconds())})
	}
	return protocol.NewMessage(&packets.DeviceSetPower{Level: math.MaxUint16})
}

// SetPowerOff sets a device power to 0.
// An optional time.Duration argument can be specified to apply a custom transition.
func SetPowerOff(d ...time.Duration) *protocol.Message {
	if len(d) > 0 {
		return protocol.NewMessage(&packets.LightSetPower{Level: 0, Duration: uint32(d[0].Milliseconds())})
	}
	return protocol.NewMessage(&packets.DeviceSetPower{Level: 0})
}

// SetColor sets a device color with no required fields which allows keeping certain
// parts of the original HSBK color.
func SetColor(h, s, b *float64, k *uint16, d time.Duration, waveform enums.LightWaveform) *protocol.Message {
	m := &packets.LightSetWaveformOptional{
		Color:    packets.LightHsbk{},
		Waveform: waveform,
		Cycles:   1.0,
		Period:   uint32(d.Milliseconds()),
	}
	if h != nil {
		m.Color.Hue = device.ConvertExternalToDeviceValue(*h, 360)
		m.SetHue = true
	}
	if s != nil {
		m.Color.Saturation = device.ConvertExternalToDeviceValue(*s, 100)
		m.SetSaturation = true
	}
	if b != nil {
		m.Color.Brightness = device.ConvertExternalToDeviceValue(*b, 100)
		m.SetBrightness = true
	}
	if k != nil {
		m.Color.Kelvin = *k
		m.SetKelvin = true
	}
	return protocol.NewMessage(m)
}

// SetLabel sets the device label.
func SetLabel(label string) (*protocol.Message, error) {
	encoded, err := encodeLabel(label)
	if err != nil {
		return nil, fmt.Errorf("set label: %w", err)
	}
	return protocol.NewMessage(&packets.DeviceSetLabel{Label: encoded}), nil
}

// SetLocation assigns the device to a location at the supplied update time.
func SetLocation(id device.LocationID, label string, updatedAt time.Time) (*protocol.Message, error) {
	encoded, err := encodeLabel(label)
	if err != nil {
		return nil, fmt.Errorf("set location: %w", err)
	}
	updatedAtNanos, err := encodeUpdatedAt(updatedAt)
	if err != nil {
		return nil, fmt.Errorf("set location: %w", err)
	}
	return protocol.NewMessage(&packets.DeviceSetLocation{
		Location:  [16]byte(id),
		Label:     encoded,
		UpdatedAt: updatedAtNanos,
	}), nil
}

// SetGroup assigns the device to a group at the supplied update time.
func SetGroup(id device.GroupID, label string, updatedAt time.Time) (*protocol.Message, error) {
	encoded, err := encodeLabel(label)
	if err != nil {
		return nil, fmt.Errorf("set group: %w", err)
	}
	updatedAtNanos, err := encodeUpdatedAt(updatedAt)
	if err != nil {
		return nil, fmt.Errorf("set group: %w", err)
	}
	return protocol.NewMessage(&packets.DeviceSetGroup{
		Group:     [16]byte(id),
		Label:     encoded,
		UpdatedAt: updatedAtNanos,
	}), nil
}

func encodeLabel(label string) ([32]byte, error) {
	var encoded [32]byte
	if !utf8.ValidString(label) {
		return encoded, fmt.Errorf("label is not valid UTF-8")
	}
	if strings.IndexByte(label, 0) >= 0 {
		return encoded, fmt.Errorf("label contains a null byte")
	}
	if len(label) > len(encoded) {
		return encoded, fmt.Errorf("label is %d bytes; maximum is %d", len(label), len(encoded))
	}
	copy(encoded[:], label)
	return encoded, nil
}

func encodeUpdatedAt(updatedAt time.Time) (uint64, error) {
	seconds := updatedAt.Unix()
	if seconds < 0 {
		return 0, fmt.Errorf("updatedAt must not precede the Unix epoch")
	}
	nanos := uint64(updatedAt.Nanosecond())
	if uint64(seconds) > (math.MaxUint64-nanos)/uint64(time.Second) {
		return 0, fmt.Errorf("updatedAt exceeds uint64 nanosecond range")
	}
	return uint64(seconds)*uint64(time.Second) + nanos, nil
}

// GetRelayPower requests the current power level for a switch relay.
func GetRelayPower(index int) *protocol.Message {
	return protocol.NewMessage(&packets.RelayGetPower{RelayIndex: uint8(index)})
}

// SetRelayPower sets a switch relay to either on or off.
func SetRelayPower(index int, poweredOn bool) *protocol.Message {
	level := uint16(0)
	if poweredOn {
		level = math.MaxUint16
	}
	return SetRelayPowerLevel(index, level)
}

// SetRelayPowerLevel sets a switch relay power level directly.
func SetRelayPowerLevel(index int, level uint16) *protocol.Message {
	return protocol.NewMessage(&packets.RelaySetPower{RelayIndex: uint8(index), Level: level})
}

// GetButtonConfig requests switch button configuration, including haptic and backlight colors.
func GetButtonConfig() *protocol.Message {
	return protocol.NewMessage(&packets.ButtonGetConfig{})
}

// SetButtonConfig sets switch haptic duration and backlight colors for on/off states.
func SetButtonConfig(hapticDurationMs uint16, backlightOn, backlightOff device.Color) *protocol.Message {
	return protocol.NewMessage(&packets.ButtonSetConfig{
		HapticDurationMs:  hapticDurationMs,
		BacklightOnColor:  buttonBacklightColor(backlightOn),
		BacklightOffColor: buttonBacklightColor(backlightOff),
	})
}

func buttonBacklightColor(c device.Color) packets.ButtonBacklightHsbk {
	dc := c.ToDeviceColor()
	return packets.ButtonBacklightHsbk(dc)
}
