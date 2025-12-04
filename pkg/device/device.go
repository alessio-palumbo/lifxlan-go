package device

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	"github.com/alessio-palumbo/lifxregistry-go/gen/registry"
)

// deviceType describes the type of LIFX device.
type deviceType int

const (
	// DeviceTypeLight is a device of type light
	DeviceTypeLight deviceType = iota
	// DeviceTypeSwitch is a device of type switch
	DeviceTypeSwitch
	// DeviceTypeHybrid is a device that act both as a light and a switch
	DeviceTypeHybrid
)

// String converts a deviceType into a string.
func (d deviceType) String() string {
	switch d {
	case DeviceTypeLight:
		return "light"
	case DeviceTypeSwitch:
		return "switch"
	case DeviceTypeHybrid:
		return "hybrid"
	}
	return ""
}

// lightType describe what interface a light implements
// and what capability it has access to.
type lightType int

const (
	// LightTypeSingleZone is a light with a single zone
	LightTypeSingleZone lightType = iota
	// LightTypeMultiZone is a light with multi_zone capability
	LightTypeMultiZone
	// LightTypeMatrix is a light with matrix capability
	LightTypeMatrix
)

// String converts a lightType into a string.
func (l lightType) String() string {
	switch l {
	case LightTypeSingleZone:
		return "single_zone"
	case LightTypeMultiZone:
		return "multi_zone"
	case LightTypeMatrix:
		return "matrix"
	}
	return ""
}

// Serial is a LIFX device serial as set in the protocol Header,
// the first 6 bytes contains the serial number and the last 2 bytes are set to 0.
type Serial [8]byte

// SerialFromHex parses an hex string into a Serial.
func SerialFromHex(hexStr string) (Serial, error) {
	if len(hexStr) != 12 {
		return Serial{}, fmt.Errorf("expected 12 hex chars (6 bytes), got %d", len(hexStr))
	}

	var b [8]byte
	_, err := hex.Decode(b[:6], []byte(hexStr))
	if err != nil {
		return Serial{}, fmt.Errorf("decode error: %v", err)
	}

	return Serial(b), nil
}

// String converts a serial into its hexadecimal equivalent.
func (s Serial) String() string {
	return fmt.Sprintf("%x", s[:6])
}

// IsNil returns whether the serial set.
func (s Serial) IsNil() bool {
	return s == [8]byte{}
}

// WifiRSSI represents either RSSI or SNR depending on firmware.
type WifiRSSI int

const (
	SignalNone      string = "No Signal"
	SignalVeryPoor  string = "Very Poor"
	SignalPoor      string = "Poor"
	SignalFair      string = "Fair"
	SignalGood      string = "Good"
	SignalExcellent string = "Excellent"
)

// String returns a description of the WifiRSSI signal
// handling both RSSI and SNR values as per LIFX docs.
func (w WifiRSSI) String() string {
	switch {
	case w < 0:
		if w >= -50 {
			return SignalExcellent
		} else if w >= -60 {
			return SignalGood
		} else if w >= -70 {
			return SignalFair
		} else if w >= -80 {
			return SignalPoor
		} else {
			return SignalVeryPoor
		}
	case w >= 4 || w <= 24:
		if w > 20 {
			return SignalExcellent
		} else if w > 16 {
			return SignalGood
		} else if w >= 12 {
			return SignalFair
		} else if w >= 7 {
			return SignalPoor
		} else {
			return SignalVeryPoor
		}
	}

	return SignalNone
}

// Device is the representation of a LIFX device on the LAN.
// Address and Serial are immutable fields while DeviceState
// fields are periodically updated.
type Device struct {
	// Immutable
	Address *net.UDPAddr
	Serial  Serial

	// Mutable

	// Low Frequency updated fields.
	Label           string
	RegistryName    string
	ProductID       uint32
	FirmwareVersion string
	Type            deviceType
	LightType       lightType
	Location        string
	Group           string
	WifiRSSI        WifiRSSI

	// Device specific properties.
	MatrixProperties    MatrixProperties
	MultizoneProperties MultizoneProperties

	// High Frequency updated fields.
	Color         Color
	PoweredOn     bool
	LastSeenAt    time.Time
	LastUpdatedAt time.Time
}

type MatrixProperties struct {
	Height      int
	Width       int
	ChainLength int
	ChainState  [][64]packets.LightHsbk
}

type MultizoneProperties struct {
	Zones []packets.LightHsbk
}

func NewDevice(address *net.UDPAddr, serial [8]byte) *Device {
	return &Device{Address: address, Serial: Serial(serial)}
}

func (d *Device) SetProductInfo(pid uint32) {
	p := registry.ProductsByPID[int(pid)]
	d.ProductID = pid
	d.RegistryName = p.Name

	if p.Features.Relays {
		d.Type = DeviceTypeSwitch
	} else if isLight(p.Features) && p.Features.Buttons {
		d.Type = DeviceTypeHybrid
	}

	if p.Features.Multizone {
		d.LightType = LightTypeMultiZone
	} else if p.Features.Matrix {
		d.LightType = LightTypeMatrix
	}
}

// SetMatrixProperties sets the matrix size and length properties
// according to the first tile in the chain.
// It also initialises the ChainState slice or resizes it according to the length.
func (d *Device) SetMatrixProperties(p *packets.TileStateDeviceChain) (updated bool) {
	if p.TileDevicesCount == 0 {
		return
	}
	firstIdx := int(p.StartIndex)
	w, h, l := int(p.TileDevices[firstIdx].Width), int(p.TileDevices[firstIdx].Height), int(p.TileDevicesCount)

	if d.MatrixProperties.Width == w && d.MatrixProperties.Height == h && d.MatrixProperties.ChainLength == l {
		return
	}

	d.MatrixProperties.Width = w
	d.MatrixProperties.Height = h
	d.MatrixProperties.ChainLength = l

	cl := len(d.MatrixProperties.ChainState)
	switch {
	case cl == 0:
		d.MatrixProperties.ChainState = make([][64]packets.LightHsbk, l)
	case cl < l:
		for range l - cl {
			d.MatrixProperties.ChainState = append(d.MatrixProperties.ChainState, [64]packets.LightHsbk{})
		}
	case cl > l:
		d.MatrixProperties.ChainState = slices.Delete(d.MatrixProperties.ChainState, l, cl)
	}

	return true
}

// SetMatrixState sets the colors of the matrix at the given index.
func (d *Device) SetMatrixState(p *packets.TileState64) (updated bool) {
	if int(p.TileIndex) > len(d.MatrixProperties.ChainState)-1 {
		return
	}

	if len(d.MatrixProperties.ChainState[p.TileIndex]) == len(p.Colors) {
		for i, c := range d.MatrixProperties.ChainState[p.TileIndex] {
			if c != p.Colors[i] {
				updated = true
				break
			}
		}
		if !updated {
			return
		}
	}

	d.MatrixProperties.ChainState[p.TileIndex] = p.Colors
	return true
}

func (d *Device) SetMultizoneProperties(p *packets.MultiZoneExtendedStateMultiZone) (updated bool) {
	if len(d.MultizoneProperties.Zones) != int(p.Count) {
		d.MultizoneProperties.Zones = make([]packets.LightHsbk, p.Count)
	}

	nZones := len(d.MultizoneProperties.Zones)
	startIndex := int(p.Index)
	if p.ColorsCount == 0 || startIndex >= nZones {
		return
	}

	copy(d.MultizoneProperties.Zones[startIndex:], p.Colors[:])
	return true
}

// HighFreqStateMessages returns a list of messages to gather state that
// change often and should be polled frequently.
// Messages differes according to device type.
// TODO Handle switches.
func (d *Device) HighFreqStateMessages() []*protocol.Message {
	switch d.LightType {
	case LightTypeMultiZone:
		return []*protocol.Message{
			protocol.NewMessage(&packets.LightGet{}),
			protocol.NewMessage(&packets.DeviceGetPower{}),
			protocol.NewMessage(&packets.MultiZoneExtendedGetColorZones{}),
		}
	case LightTypeMatrix:
		return []*protocol.Message{
			protocol.NewMessage(&packets.LightGet{}),
			protocol.NewMessage(&packets.DeviceGetPower{}),
			protocol.NewMessage(&packets.TileGet64{
				Length: uint8(d.MatrixProperties.ChainLength),
				Rect:   packets.TileBufferRect{Width: uint8(d.MatrixProperties.Width)},
			}),
		}
	default:
		return []*protocol.Message{protocol.NewMessage(&packets.LightGet{})}
	}
}

// LowFreqStateMessages returns a list of messages to gather state that
// does not change often and should be polled less frequently.
// Messages differes according to device type.
// TODO Handle switches.
func (d *Device) LowFreqStateMessages() []*protocol.Message {
	msg := []*protocol.Message{
		protocol.NewMessage(&packets.DeviceGetLabel{}),
		protocol.NewMessage(&packets.DeviceGetHostFirmware{}),
		protocol.NewMessage(&packets.DeviceGetLocation{}),
		protocol.NewMessage(&packets.DeviceGetGroup{}),
		protocol.NewMessage(&packets.DeviceGetWifiInfo{}),
	}

	if d.LightType == LightTypeMatrix {
		msg = append(msg, protocol.NewMessage(&packets.TileGetDeviceChain{}))
	}
	return msg
}

// SortDevices sorts devices by label and if equal, by Serial.
func SortDevices(devices []Device) {
	slices.SortFunc(devices, func(a, b Device) int {
		if n := strings.Compare(a.Label, b.Label); n != 0 {
			return n
		}
		// If names are equal, order by serial
		return bytes.Compare(a.Serial[:], b.Serial[:])
	})
}

// ParseLabel parses the raw byte label into a string and trims C-style null bytes.
func ParseLabel(label [32]byte) string {
	return strings.Trim(string(label[:]), "\u0000")
}

func isLight(f registry.FeatureSet) bool {
	return f.Color || f.TemperatureRange != nil
}
