package messages

import (
	"math"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/enums"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

const (
	defaultPeriod = time.Second
)

// SetPowerOn sets a device power to its maximum value of 65535.
func SetPowerOn() *protocol.Message {
	return protocol.NewMessage(&packets.DeviceSetPower{Level: math.MaxUint16})
}

// SetPowerOff sets a device power to 0.
func SetPowerOff() *protocol.Message {
	return protocol.NewMessage(&packets.DeviceSetPower{Level: 0})
}

// SetColor sets a device color with no required fields which allows keeping certain
// parts of the original HSBK color.
func SetColor(h, s, b *float64, k *uint16, d time.Duration, waveform enums.LightWaveform) *protocol.Message {
	if d < time.Second {
		d = defaultPeriod
	}
	m := &packets.LightSetWaveformOptional{
		Color:    packets.LightHsbk{},
		Waveform: waveform,
		Cycles:   1.0,
		Period:   uint32(d.Milliseconds()),
	}
	if h != nil {
		m.Color.Hue = convertExternalToDeviceValue(*h, 360)
		m.SetHue = true
	}
	if s != nil {
		m.Color.Saturation = convertExternalToDeviceValue(*s, 100)
		m.SetSaturation = true
	}
	if b != nil {
		m.Color.Brightness = convertExternalToDeviceValue(*b, 100)
		m.SetBrightness = true
	}
	if k != nil {
		m.Color.Kelvin = *k
		m.SetKelvin = true
	}
	return protocol.NewMessage(m)
}

func SetMatrixColors(startIndex, length, width int, colors [64]packets.LightHsbk, d time.Duration) *protocol.Message {
	m := &packets.TileSet64{
		TileIndex: uint8(startIndex),
		Length:    uint8(length),
		Rect:      packets.TileBufferRect{Width: uint8(width)},
		Duration:  uint32(d.Milliseconds()),
		Colors:    colors,
	}
	return protocol.NewMessage(m)
}

// convertExternalToDeviceValue takes an external value and multiplier
// and converts it into a device value 0-65535.
func convertExternalToDeviceValue(v float64, multiplier float64) uint16 {
	return uint16(math.Round(v * math.MaxUint16 / multiplier))
}
