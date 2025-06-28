package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

var (
	TargetBroadcast = [8]byte{}
)

type Message struct {
	Header  Header
	Payload packets.Payload
}

func NewMessage(payload packets.Payload) *Message {
	var h Header
	h.Size = uint16(HeaderSize + payload.Size())
	h.SetProtocol(lifxProtocol)
	h.SetAddressable(true)
	h.SetOrigin(0)
	h.Type = payload.PayloadType()

	return &Message{
		Header:  h,
		Payload: payload,
	}
}

func (m *Message) SetSource(source uint32) {
	m.Header.Source = source
}

func (m *Message) SetSequence(seq uint8) {
	m.Header.Sequence = seq
}

func (m *Message) SetTarget(target [8]byte) {
	m.Header.Target = target
	m.Header.SetTagged(target == TargetBroadcast)
}

func (m *Message) SetAckRequired(v bool) {
	m.Header.SetAckRequired(v)
}

func (m *Message) SetResponseRequired(v bool) {
	m.Header.SetResponseRequired(v)
}

func (m *Message) String() string {
	return fmt.Sprintf("Message{Type: %d, Size: %d, Payload: %#v}", m.Header.Type, m.Header.Size, m.Payload)
}

func (m *Message) MarshalBinary() ([]byte, error) {
	if m.Payload == nil {
		return nil, fmt.Errorf("cannot marshal message with nil payload")
	}

	payloadBytes, err := m.Payload.MarshalBinary()
	if err != nil {
		return nil, err
	}

	m.Header.Type = m.Payload.PayloadType()
	m.Header.Size = uint16(len(payloadBytes) + HeaderSize)

	var buf bytes.Buffer

	if err := binary.Write(&buf, binary.LittleEndian, m.Header); err != nil {
		return nil, err
	}
	buf.Write(payloadBytes)

	return buf.Bytes(), nil
}

func (m *Message) UnmarshalBinary(data []byte) error {
	if len(data) < HeaderSize {
		return fmt.Errorf("data too short: got %d, want at least %d", len(data), HeaderSize)
	}

	if err := binary.Read(bytes.NewReader(data[:HeaderSize]), binary.LittleEndian, &m.Header); err != nil {
		return err
	}

	payloadType := m.Header.Type
	newPayload, ok := packets.Payloads[payloadType]
	if !ok {
		return fmt.Errorf("unknown payload type: %d", payloadType)
	}

	payload := newPayload()
	if err := payload.UnmarshalBinary(data[HeaderSize:]); err != nil {
		return err
	}

	m.Payload = payload
	return nil
}
