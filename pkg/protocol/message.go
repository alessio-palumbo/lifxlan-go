package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/alessio-palumbo/lifxlan-go/internal/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

const lifxProtocol = 1024

// TargetBroadcast marks the message as a broadcast message.
var TargetBroadcast = [8]byte{}

// Message represents a LIFX LAN protocol message.
type Message struct {
	header  protocol.Header
	Payload packets.Payload
}

// NewMessage returns a new Message with the given payload.
func NewMessage(payload packets.Payload) *Message {
	var h protocol.Header
	h.Size = uint16(protocol.HeaderSize + payload.Size())
	h.SetProtocol(lifxProtocol)
	h.SetAddressable(true)
	h.SetOrigin(0)
	h.Type = payload.PayloadType()

	return &Message{
		header:  h,
		Payload: payload,
	}
}

// Type returns the Payload type set in the header.
func (m *Message) Type() uint16 {
	return m.header.Type
}

// Source returns the Message source in the header.
func (m *Message) Source() uint32 {
	return m.header.Source
}

// SetSource sets the source of the message, which is
// sent back in the device response.
func (m *Message) SetSource(source uint32) {
	m.header.Source = source
}

// Sequence returns the sequence set in the Message header.
func (m *Message) Sequence() uint8 {
	return m.header.Sequence
}

// SetSequence sets the sequence of a Message which can be use to track message order.
func (m *Message) SetSequence(seq uint8) {
	m.header.Sequence = seq
}

// Target returns the target set in the Message header.
func (m *Message) Target() [8]byte {
	return m.header.Target
}

// SetTarget sets the target device of a message.
// For broadcasts messages target is an empty [8]byte.
func (m *Message) SetTarget(target [8]byte) {
	m.header.Target = target
	m.header.SetTagged(target == TargetBroadcast)
}

// SetAckRequired sets whether an Ack is required.
func (m *Message) SetAckRequired(v bool) {
	m.header.SetAckRequired(v)
}

// SetResponseRequired sets whether a response is required.
func (m *Message) SetResponseRequired(v bool) {
	m.header.SetResponseRequired(v)
}

// String implements Stringer interface for easy logging.
func (m *Message) String() string {
	return fmt.Sprintf("Message{Type: %d, Size: %d, Payload: %#v}", m.header.Type, m.header.Size, m.Payload)
}

// MarshalBinary encodes the Message into its binary wire format.
func (m *Message) MarshalBinary() ([]byte, error) {
	if m.Payload == nil {
		return nil, fmt.Errorf("cannot marshal message with nil payload")
	}

	payloadBytes, err := m.Payload.MarshalBinary()
	if err != nil {
		return nil, err
	}

	m.header.Type = m.Payload.PayloadType()
	m.header.Size = uint16(len(payloadBytes) + protocol.HeaderSize)

	var buf bytes.Buffer

	if err := binary.Write(&buf, binary.LittleEndian, m.header); err != nil {
		return nil, err
	}
	buf.Write(payloadBytes)

	return buf.Bytes(), nil
}

// UnmarshalBinary decodes a message from its binary wire format.
func (m *Message) UnmarshalBinary(data []byte) error {
	hSize := protocol.HeaderSize
	if len(data) < hSize {
		return fmt.Errorf("data too short: got %d, want at least %d", len(data), hSize)
	}

	if err := binary.Read(bytes.NewReader(data[:hSize]), binary.LittleEndian, &m.header); err != nil {
		return err
	}

	payloadType := m.header.Type
	newPayload, ok := packets.Payloads[payloadType]
	if !ok {
		return fmt.Errorf("unknown payload type: %d", payloadType)
	}

	payload := newPayload()
	if err := payload.UnmarshalBinary(data[hSize:]); err != nil {
		return err
	}

	m.Payload = payload
	return nil
}
