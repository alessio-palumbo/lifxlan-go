package protocol

import (
	"encoding/binary"
	"errors"
)

const (
	HeaderSize   = 36
	lifxProtocol = 1024
)

var ErrInvalidHeaderLength = errors.New("invalid LIFX header length")

// Header represents a full 36-byte LIFX message header.
type Header struct {
	// Frame (bytes 0-7)
	Size       uint16 // 0–1: size of the entire message
	FrameFlags uint16 // 2–3: contains protocol (12 bits), addressable (1bit), tagged (1bit), origin (2bits)
	Source     uint32 // 4–7

	// Frame Address (bytes 8–23)
	Target    [8]byte // 8–15 (MAC address or 0 for broadcast)
	Reserved1 [6]byte // 16–21 (zero)
	AddrFlags uint8   // 22: res_required (1bit), ack_required (1bit), reserved (6bits)
	Sequence  uint8   // 23

	// Protocol Header (bytes 24–35)
	Reserved2 [8]byte // 24–31 (zero)
	Type      uint16  // 32–33: determines the payload being used
	Reserved3 uint16  // 34–35 (zero)
}

// Protocol returns the 12-bit protocol field from the FrameFlags.
// This should typically be 1024 for LIFX protocol messages.
func (h *Header) Protocol() uint16 {
	return h.FrameFlags & 0x0FFF
}

// SetProtocol sets the 12-bit protocol field in the FrameFlags.
func (h *Header) SetProtocol(p uint16) {
	h.FrameFlags = (h.FrameFlags & 0xF000) | (p & 0x0FFF)
}

// IsAddressable returns true if the message includes a target address.
// This corresponds to the addressable bit (bit 12) in FrameFlags.
func (h *Header) IsAddressable() bool {
	return (h.FrameFlags>>12)&0x1 == 1
}

// SetAddressable sets or clears the addressable bit (bit 12) in FrameFlags.
func (h *Header) SetAddressable(v bool) {
	if v {
		h.FrameFlags |= (1 << 12)
	} else {
		h.FrameFlags &^= (1 << 12)
	}
}

// IsTagged returns true if the tagged bit (bit 13) is set in FrameFlags.
func (h *Header) IsTagged() bool {
	return (h.FrameFlags>>13)&0x1 == 1
}

// SetTagged sets or clears the tagged bit (bit 13) in FrameFlags.
// Tagged should be true for broadcast messages and false for unicast.
func (h *Header) SetTagged(v bool) {
	if v {
		h.FrameFlags |= (1 << 13)
	} else {
		h.FrameFlags &^= (1 << 13)
	}
}

// Origin returns the 2-bit origin field from FrameFlags (bits 14–15).
// This is generally 0 in most applications.
func (h *Header) Origin() uint8 {
	return uint8((h.FrameFlags >> 14) & 0x3)
}

// SetOrigin sets the 2-bit origin field (bits 14–15) in FrameFlags.
// This field is rarely used and should typically be 0.
func (h *Header) SetOrigin(o uint8) {
	h.FrameFlags = (h.FrameFlags & 0x3FFF) | (uint16(o&0x3) << 14)
}

// AckRequired returns true if the ack_required flag (bit 1) is set in AddrFlags.
func (h *Header) AckRequired() bool {
	return h.AddrFlags&0x2 != 0
}

// SetAckRequired sets or clears the ack_required flag (bit 1) in AddrFlags.
func (h *Header) SetAckRequired(v bool) {
	if v {
		h.AddrFlags |= 0x2
	} else {
		h.AddrFlags &^= 0x2
	}
}

// ResponseRequired returns true if the res_required flag (bit 0) is set in AddrFlags.
func (h *Header) ResponseRequired() bool {
	return h.AddrFlags&0x1 != 0
}

// SetResponseRequired sets or clears the res_required flag (bit 0) in AddrFlags.
// Set to true to explicitly request a State message response from the device.
func (h *Header) SetResponseRequired(v bool) {
	if v {
		h.AddrFlags |= 0x1
	} else {
		h.AddrFlags &^= 0x1
	}
}

func (h *Header) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 36)
	binary.LittleEndian.PutUint16(buf[0:], h.Size)
	binary.LittleEndian.PutUint16(buf[2:], h.FrameFlags)
	binary.LittleEndian.PutUint32(buf[4:], h.Source)
	copy(buf[8:], h.Target[:])
	copy(buf[16:], h.Reserved1[:])
	buf[22] = h.AddrFlags
	buf[23] = h.Sequence
	copy(buf[24:], h.Reserved2[:])
	binary.LittleEndian.PutUint16(buf[32:], h.Type)
	binary.LittleEndian.PutUint16(buf[34:], h.Reserved3)
	return buf, nil
}

func (h *Header) UnmarshalBinary(data []byte) error {
	if len(data) < 36 {
		return ErrInvalidHeaderLength
	}
	h.Size = binary.LittleEndian.Uint16(data[0:])
	h.FrameFlags = binary.LittleEndian.Uint16(data[2:])
	h.Source = binary.LittleEndian.Uint32(data[4:])
	copy(h.Target[:], data[8:16])
	copy(h.Reserved1[:], data[16:22])
	h.AddrFlags = data[22]
	h.Sequence = data[23]
	copy(h.Reserved2[:], data[24:32])
	h.Type = binary.LittleEndian.Uint16(data[32:])
	h.Reserved3 = binary.LittleEndian.Uint16(data[34:])
	return nil
}
