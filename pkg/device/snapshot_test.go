package device

import (
	"reflect"
	"testing"

	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func TestNewDeviceStateSnapshotCopiesRestorableState(t *testing.T) {
	serial := mustSerial(t, "001122334455")
	zoneColor := packets.LightHsbk{Hue: 1, Saturation: 2, Brightness: 3, Kelvin: 3500}
	chainColor := packets.LightHsbk{Hue: 4, Saturation: 5, Brightness: 6, Kelvin: 4000}
	d := Device{
		Serial:    serial,
		PoweredOn: true,
		Color:     Color{Hue: 120, Saturation: 80, Brightness: 60, Kelvin: 3500},
		LightType: LightTypeMatrix,
		MultizoneProperties: MultizoneProperties{
			Zones: []packets.LightHsbk{zoneColor},
		},
		MatrixProperties: MatrixProperties{
			Width:      8,
			ChainZones: [][]packets.LightHsbk{{chainColor}},
		},
	}

	got := NewDeviceStateSnapshot(d)

	if got.Serial != serial || !got.PoweredOn || got.Color != d.Color || got.LightType != LightTypeMatrix || got.MatrixWidth != 8 {
		t.Fatalf("snapshot = %#v", got)
	}
	if !reflect.DeepEqual(got.Zones, []packets.LightHsbk{zoneColor}) {
		t.Fatalf("zones = %#v", got.Zones)
	}
	if !reflect.DeepEqual(got.MatrixChains, [][]packets.LightHsbk{{chainColor}}) {
		t.Fatalf("matrix chains = %#v", got.MatrixChains)
	}

	d.MultizoneProperties.Zones[0].Hue = 99
	d.MatrixProperties.ChainZones[0][0].Hue = 98
	if got.Zones[0].Hue != zoneColor.Hue {
		t.Fatal("snapshot shares multizone backing slice")
	}
	if got.MatrixChains[0][0].Hue != chainColor.Hue {
		t.Fatal("snapshot shares matrix backing slice")
	}
}

func TestNewStateSnapshotCopiesDevices(t *testing.T) {
	devices := []Device{
		{Serial: mustSerial(t, "001122334455"), Color: Color{Hue: 10}},
		{Serial: mustSerial(t, "aabbccddeeff"), Color: Color{Hue: 20}},
	}

	got := NewStateSnapshot(devices)

	if len(got.Devices) != 2 {
		t.Fatalf("devices = %d, want 2", len(got.Devices))
	}
	if got.Devices[0].Serial != devices[0].Serial || got.Devices[1].Serial != devices[1].Serial {
		t.Fatalf("snapshot devices = %#v", got.Devices)
	}
}

func TestRestorableStateMessagesRequestMatrixChainBeforePixels(t *testing.T) {
	d := Device{LightType: LightTypeMatrix}

	got := RestorableStateMessages(d)

	if !hasPayload(got, uint16(packets.PayloadTypeTileGetDeviceChain)) {
		t.Fatalf("messages should request matrix chain metadata before pixels")
	}
	if hasPayload(got, uint16(packets.PayloadTypeTileGet64)) {
		t.Fatalf("messages should not request matrix pixels without chain metadata")
	}
}

func TestRestorableStateMessagesRequestMatrixPixels(t *testing.T) {
	d := Device{LightType: LightTypeMatrix}
	d.MatrixProperties.Width = 8
	d.MatrixProperties.Height = 8
	d.MatrixProperties.ChainLength = 2
	d.MatrixProperties.StatePackets = 1

	got := RestorableStateMessages(d)

	if !hasPayload(got, uint16(packets.PayloadTypeTileGet64)) {
		t.Fatalf("messages should request matrix pixels when chain metadata is known")
	}
	if countPayload(got, uint16(packets.PayloadTypeTileGet64)) != 2 {
		t.Fatalf("TileGet64 message count = %d, want 2", countPayload(got, uint16(packets.PayloadTypeTileGet64)))
	}
}

func TestRestorableStateReadyRequiresPoweredOnMatrixBuffers(t *testing.T) {
	d := Device{
		Serial:    mustSerial(t, "001122334455"),
		LightType: LightTypeMatrix,
		PoweredOn: true,
	}
	d.MatrixProperties.Width = 8
	d.MatrixProperties.Height = 8
	d.MatrixProperties.ChainLength = 1

	if RestorableStateReady(d) {
		t.Fatal("matrix state should not be ready without chain colors")
	}

	d.MatrixProperties.ChainZones = [][]packets.LightHsbk{{{}, {}}}
	if !RestorableStateReady(d) {
		t.Fatal("matrix state should be ready with captured chain colors, even when empty")
	}
}

func TestRestorableStateReadyIgnoresPoweredOffMatrixBuffers(t *testing.T) {
	d := Device{
		Serial:    mustSerial(t, "001122334455"),
		LightType: LightTypeMatrix,
		PoweredOn: false,
	}
	d.MatrixProperties.ChainLength = 1

	if !RestorableStateReady(d) {
		t.Fatal("powered-off matrix should not block snapshot readiness")
	}
}

func TestCloneHSBKsAndMatrixChains(t *testing.T) {
	zeroColors := []packets.LightHsbk{{}, {}}
	if got := CloneHSBKs(zeroColors); len(got) != len(zeroColors) {
		t.Fatalf("CloneHSBKs zero buffer len = %d, want %d", len(got), len(zeroColors))
	}

	chains := [][]packets.LightHsbk{
		{{Hue: 1, Saturation: 2, Brightness: 3, Kelvin: 3500}},
		zeroColors,
	}
	got := CloneMatrixChains(chains)
	chains[0][0].Hue = 99
	chains[1][0].Hue = 98

	if got[0][0].Hue != 1 {
		t.Fatalf("cloned hue = %d, want original 1", got[0][0].Hue)
	}
	if len(got[1]) != len(zeroColors) || got[1][0].Hue != 0 {
		t.Fatalf("cloned zero buffer = %#v", got[1])
	}
}

func hasPayload(messages []*protocol.Message, payloadType uint16) bool {
	return countPayload(messages, payloadType) > 0
}

func countPayload(messages []*protocol.Message, payloadType uint16) int {
	count := 0
	for _, msg := range messages {
		if msg.Type() == payloadType {
			count++
		}
	}
	return count
}
