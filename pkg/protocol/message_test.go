package protocol

import (
	"testing"

	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func TestMessage_MarshalUnmarshal(t *testing.T) {
	payload := &packets.LightSetColor{
		Color: packets.LightHsbk{
			Hue:        21845,
			Saturation: 65535,
			Brightness: 65535,
			Kelvin:     3500,
		},
		Duration: 0,
	}
	original := NewMessage(payload)
	original.SetTarget([8]byte{0xd0, 0x73, 0xd5, 0x00, 0x13, 0x37})
	original.SetSource(1234)

	data, err := original.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary failed: %v", err)
	}

	var decoded Message
	err = decoded.UnmarshalBinary(data)
	if err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}

	// Assert header round-trip
	if original.Type() != decoded.Type() ||
		original.Source() != decoded.Source() ||
		original.Sequence() != decoded.Sequence() {
		t.Errorf("Header mismatch: got %+v, want %+v", decoded, original)
	}

	// Assert payload type and values
	gotPayload, ok := decoded.Payload.(*packets.LightSetColor)
	if !ok {
		t.Fatalf("Decoded payload has wrong type: %T", decoded.Payload)
	}

	wantPayload := original.Payload.(*packets.LightSetColor)
	if *gotPayload != *wantPayload {
		t.Errorf("Payload mismatch:\n got: %#v\nwant: %#v", gotPayload, wantPayload)
	}
}
