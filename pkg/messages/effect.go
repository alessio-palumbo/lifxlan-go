package messages

import (
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

// EffectInstanceID returns the instance ID carried by a matrix or multizone
// set-effect message. The boolean is false for other message types.
//
// Consumers can record the ID before sending the message and compare it with
// subsequently observed device effect state. The ID is a correlation token,
// not proof that no other controller issued the effect.
func EffectInstanceID(msg *protocol.Message) (uint32, bool) {
	if msg == nil {
		return 0, false
	}

	switch payload := msg.Payload.(type) {
	case *packets.MultiZoneSetEffect:
		return payload.Settings.Instanceid, true
	case *packets.TileSetEffect:
		return payload.Settings.Instanceid, true
	default:
		return 0, false
	}
}
