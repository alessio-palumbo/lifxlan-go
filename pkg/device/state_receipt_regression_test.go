package device

import (
	"testing"

	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func TestMatrixStateOffsetDoesNotWrapAt256Pixels(t *testing.T) {
	d := &Device{MatrixProperties: MatrixProperties{
		Width: 16, Height: 20, ChainLength: 1,
		ChainZones: [][]packets.LightHsbk{make([]packets.LightHsbk, 320)},
	}}
	color := packets.LightHsbk{Brightness: 123, Kelvin: 3500}
	d.SetMatrixState(&packets.TileState64{
		Rect:   packets.TileBufferRect{Width: 16, Y: 16},
		Colors: [64]packets.LightHsbk{color},
	})
	if d.MatrixProperties.ChainZones[0][256] != color || d.MatrixProperties.ChainZones[0][0] != (packets.LightHsbk{}) {
		t.Fatal("matrix response used an overflowing pixel offset")
	}
}

func TestMultizoneResponseCopiesOnlyDeclaredColors(t *testing.T) {
	color := packets.LightHsbk{Brightness: 123, Kelvin: 3500}
	d := &Device{MultizoneProperties: MultizoneProperties{Zones: []packets.LightHsbk{color, color, color}}}
	d.SetMultizoneProperties(&packets.MultiZoneExtendedStateMultiZone{Count: 3, ColorsCount: 1})
	if d.MultizoneProperties.Zones[0] != (packets.LightHsbk{}) ||
		d.MultizoneProperties.Zones[1] != color || d.MultizoneProperties.Zones[2] != color {
		t.Fatal("partial response overwrote colors outside its declared range")
	}
}
