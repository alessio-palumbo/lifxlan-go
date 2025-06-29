package controller

import (
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/internal/protocol"
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
type lightType string

const (
	// LightTypeSingleZone is a light with a single zone
	LightTypeSingleZone lightType = "single_zone"
	// LightTypeMultiZone is a light with multi_zone capability
	LightTypeMultiZone lightType = "multi_zone"
	// LightTypeMatrix is a light with matrix capability
	LightTypeMatrix lightType = "matrix"
)

// Serial is a LIFX device serial as set in the protocol Header,
// the first 6 bytes contains the serial number and the last 2 bytes are set to 0.
type Serial [8]byte

// String converts a serial into its hexadecimal equivalent.
func (s Serial) String() string {
	return fmt.Sprintf("%x", s[:6])
}

// IsNil returns whether the serial set.
func (s Serial) IsNil() bool {
	return s == [8]byte{}
}

// Device contains the UDP address and state of a LIFX device on the LAN.
type Device struct {
	Address         *net.UDPAddr
	Serial          Serial
	Label           string
	RegistryName    string
	ProductID       uint32
	FirmwareVersion string
	Type            deviceType
	LightType       lightType
	Location        string
	Group           string
	Color           Color
	PoweredOn       bool
	LastSeenAt      time.Time
}

func NewDevice(address *net.UDPAddr, serial [8]byte) *Device {
	return &Device{Address: address, Serial: Serial(serial)}
}

func (d *Device) SetProductID(pid uint32) {
	p := registry.ProductsByPID[int(pid)]
	d.ProductID = pid
	d.RegistryName = p.Name

	if p.Features.Relays {
		d.Type = DeviceTypeSwitch
	} else if p.Features.Buttons {
		d.Type = DeviceTypeHybrid
	}

	if p.Features.Multizone {
		d.LightType = LightTypeMultiZone
	} else if p.Features.Matrix {
		d.LightType = LightTypeMatrix
	}

}

// SortDevices sorts devices by label and if equal, by Serial.
func SortDevices(devices []Device) {
	slices.SortFunc(devices, func(a, b Device) int {
		if n := strings.Compare(a.Label, b.Label); n != 0 {
			return n
		}
		// If names are equal, order by serial
		return strings.Compare(a.Serial.String(), b.Serial.String())
	})
}

// DeviceStateMessages
func DeviceStateMessages() []*protocol.Message {
	return []*protocol.Message{
		protocol.NewMessage(&packets.DeviceGetLabel{}),
		protocol.NewMessage(&packets.DeviceGetVersion{}),
		protocol.NewMessage(&packets.LightGet{}),
		protocol.NewMessage(&packets.DeviceGetHostFirmware{}),
		protocol.NewMessage(&packets.DeviceGetLocation{}),
		protocol.NewMessage(&packets.DeviceGetGroup{}),
	}
}

// ParseLabel parses the raw byte label into a string and trims C-style null bytes.
func ParseLabel(label [32]byte) string {
	return strings.Trim(string(label[:]), "\u0000")
}
