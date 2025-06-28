package client

import (
	"fmt"
	"net"
	"slices"
	"strings"

	"github.com/alessio-palumbo/lifxlan-go/internal/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	"github.com/alessio-palumbo/lifxregistry-go/gen/registry"
)

type deviceType int

const (
	DeviceTypeLight deviceType = iota
	DeviceTypeSwitch
	DeviceTypeHybrid
)

func (d deviceType) String() string {
	switch d {
	case DeviceTypeLight:
		return "light"
	case DeviceTypeSwitch:
		return "switch"
	}
	return ""
}

type lightType string

const (
	LightTypeSingleZone lightType = "single_zone"
	LightTypeMultiZone  lightType = "multi_zone"
	LightTypeMatrix     lightType = "matrix"
)

type Serial [8]byte

func (s Serial) String() string {
	return fmt.Sprintf("%x", s[:6])
}

func (s Serial) IsNil() bool {
	return s == [8]byte{}
}

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

func SortDevices(devices []Device) {
	slices.SortFunc(devices, func(a, b Device) int {
		if n := strings.Compare(a.Label, b.Label); n != 0 {
			return n
		}
		// If names are equal, order by serial
		return strings.Compare(a.Serial.String(), b.Serial.String())
	})
}

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
