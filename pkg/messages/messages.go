package messages

import (
	"math"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
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

// SetMatrixColors returns a TileSet64 Message that sets a matrix with the given size to the provided colors.
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

const extendedMultizoneMsgMaxZones = 82

// SetMultizoneExtendedColors accepts a variable length list of colors and returns multiple MultiZoneExtendedSetColorZones
// messages to cater for devices with more than the message maximum supported zones.
// If a single message is needed than the Apply directive is set on the message itself, otherwise an extra message
// is produced to Apply the colors previously buffered by the device.
// The startIndex refers to the zone the colors should apply from, if set to 0 colors will be applied from the first zone.
func SetMultizoneExtendedColors(startIndex int, colors []packets.LightHsbk, d time.Duration) []*protocol.Message {
	var msgs []*protocol.Message
	nColors := len(colors)

	for offset := 0; offset < nColors; offset += extendedMultizoneMsgMaxZones {
		end := min(nColors, offset+extendedMultizoneMsgMaxZones)
		chunk := colors[offset:end]

		m := &packets.MultiZoneExtendedSetColorZones{
			Index:       uint16(startIndex + offset),
			ColorsCount: uint8(len(chunk)),
			Duration:    uint32(d.Milliseconds()),
		}
		copy(m.Colors[:], chunk)
		msgs = append(msgs, protocol.NewMessage(m))
	}

	if len(msgs) == 1 {
		msgs[0].Payload.(*packets.MultiZoneExtendedSetColorZones).Apply = enums.MultiZoneExtendedApplicationRequest(enums.MultiZoneApplicationRequestMULTIZONEAPPLICATIONREQUESTAPPLY)
	} else {
		m := &packets.MultiZoneExtendedSetColorZones{Apply: enums.MultiZoneExtendedApplicationRequestMULTIZONEEXTENDEDAPPLICATIONREQUESTAPPLYONLY}
		msgs = append(msgs, protocol.NewMessage(m))
	}

	return msgs
}
