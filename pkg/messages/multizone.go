package messages

import (
	"math/rand"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/enums"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

const (
	extendedMultizoneMsgMaxZones = 82
)

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

// SetMultizoneEffectOff returns a message instructing the device to turn any running multizone effect off.
func SetMultizoneEffectOff() *protocol.Message {
	return protocol.NewMessage(&packets.MultiZoneSetEffect{
		Settings: packets.MultiZoneEffectSettings{
			Instanceid: rand.Uint32(),
			Type:       enums.MultiZoneEffectTypeMULTIZONEEFFECTTYPEOFF,
		},
	})
}

// SetMultizoneMoveEffect returns a message instructing the device to run the Move effect.
func SetMultizoneMoveEffect(speed time.Duration, directionForward bool) *protocol.Message {
	var p packets.MultiZoneEffectParameter
	if directionForward {
		p.Parameter1 = 1
	}

	m := &packets.MultiZoneSetEffect{
		Settings: packets.MultiZoneEffectSettings{
			Instanceid: rand.Uint32(),
			Type:       enums.MultiZoneEffectTypeMULTIZONEEFFECTTYPEMOVE,
			Speed:      uint32(speed.Milliseconds()),
			Parameter:  p,
		},
	}
	return protocol.NewMessage(m)
}
