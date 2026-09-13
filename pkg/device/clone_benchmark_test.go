package device

import (
	"net"
	"testing"

	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

var benchmarkCloneSink Device

func BenchmarkDeviceClone(b *testing.B) {
	benchmarks := map[string]Device{
		"single_zone":  benchmarkCloneDevice(LightTypeSingleZone, 0),
		"multizone_82": benchmarkCloneDevice(LightTypeMultiZone, 82),
		"matrix_5x64":  benchmarkCloneDevice(LightTypeMatrix, 5*64),
		"switch_4":     benchmarkCloneSwitch(4),
	}

	for name, d := range benchmarks {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				benchmarkCloneSink = d.Clone()
			}
		})
	}
}

func benchmarkCloneDevice(lightType LightType, zones int) Device {
	d := Device{
		Address:      &net.UDPAddr{IP: net.IPv4(192, 168, 1, 20), Port: 56700},
		Serial:       Serial{0xd0, 0x73, 0xd5, 0x00, 0x00, byte(zones)},
		Label:        "Living Room",
		RegistryName: "LIFX benchmark fixture",
		LocationID:   LocationID{1},
		Location:     "Home",
		GroupID:      GroupID{2},
		Group:        "Downstairs",
		Type:         DeviceTypeLight,
		LightType:    lightType,
		Buttons: []Button{
			{Actions: []packets.ButtonAction{{}, {}, {}}},
		},
	}

	switch lightType {
	case LightTypeMultiZone:
		d.MultizoneProperties.Zones = make([]packets.LightHsbk, zones)
	case LightTypeMatrix:
		const zonesPerPanel = 64
		panels := zones / zonesPerPanel
		d.MatrixProperties = MatrixProperties{
			Width:             8,
			Height:            8,
			NZones:            zonesPerPanel,
			StatePackets:      1,
			ChainLength:       panels,
			ChainZones:        make([][]packets.LightHsbk, panels),
			ChainOrientations: make([]Orientation, panels),
		}
		for i := range d.MatrixProperties.ChainZones {
			d.MatrixProperties.ChainZones[i] = make([]packets.LightHsbk, zonesPerPanel)
		}
	}

	return d
}

func benchmarkCloneSwitch(relays int) Device {
	d := benchmarkCloneDevice(LightTypeSingleZone, 0)
	d.Type = DeviceTypeSwitch
	d.Relays = make([]Relay, relays)
	for i := range d.Relays {
		d.Relays[i] = Relay{Index: i, PoweredOn: i%2 == 0}
	}
	return d
}
