package messages

import (
	"math"
	"time"

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
