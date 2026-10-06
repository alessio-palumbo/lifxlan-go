package device

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func TestVisibleZoneCountMatrixMatchesSurface(t *testing.T) {
	for _, test := range []struct {
		pid                                    uint32
		width, height, pixels, chains, visible int
	}{
		{215, 5, 6, 30, 1, 27}, {215, 5, 6, 30, 2, 54},
		{57, 5, 11, 55, 1, 52}, {145, 8, 8, 64, 1, 56},
		{201, 8, 16, 128, 1, 120}, {219, 7, 5, 35, 1, 31},
		{55, 8, 8, 64, 5, 320},
	} {
		for _, buffers := range []bool{false, true} {
			d := Device{Type: DeviceTypeLight, LightType: LightTypeMatrix, ProductID: test.pid,
				MatrixProperties: MatrixProperties{Width: test.width, Height: test.height, NZones: test.pixels, ChainLength: test.chains}}
			if buffers {
				d.Type = DeviceTypeHybrid
				for range test.chains {
					d.MatrixProperties.ChainZones = append(d.MatrixProperties.ChainZones, make([]packets.LightHsbk, test.pixels))
					d.MatrixProperties.ChainOrientations = append(d.MatrixProperties.ChainOrientations, OrientationLeft)
				}
			}
			before := d.Clone()
			count, known := VisibleZoneCount(d)
			if !known || count != test.visible {
				t.Fatalf("pid=%d buffers=%v got=%d/%v want=%d", test.pid, buffers, count, known, test.visible)
			}
			visible := 0
			for _, chain := range SurfaceFromDevice(d).Matrix.Chains {
				for _, row := range chain.Rows {
					visible += row.Cols - len(row.HiddenCols)
				}
			}
			if count != visible || !reflect.DeepEqual(d, before) {
				t.Fatal("surface mismatch or mutation")
			}
		}
	}
}

func TestVisibleZoneCountSingleMultizoneAndUnknown(t *testing.T) {
	for _, test := range []struct {
		d     Device
		count int
		known bool
	}{
		{Device{}, 0, false},
		{Device{Type: DeviceTypeSwitch, ProductID: 27}, 0, false},
		{Device{Type: DeviceTypeLight, ProductID: 27, LightType: LightTypeSingleZone}, 1, true},
		{Device{Type: DeviceTypeLight, LightType: LightTypeSingleZone}, 0, false},
		{Device{Type: DeviceTypeLight, LightType: LightTypeMultiZone}, 0, false},
		{Device{Type: DeviceTypeHybrid, LightType: LightTypeMultiZone, MultizoneProperties: MultizoneProperties{Zones: make([]packets.LightHsbk, 34)}}, 34, true},
		{Device{Type: DeviceTypeLight, LightType: LightTypeMatrix}, 0, false},
		{Device{Type: DeviceTypeLight, LightType: LightType(255)}, 0, false},
	} {
		count, known := VisibleZoneCount(test.d)
		if count != test.count || known != test.known {
			t.Fatalf("got=%d/%v want=%d/%v", count, known, test.count, test.known)
		}
	}
}

func TestVisibleZoneCountChainGeometryAndOverflow(t *testing.T) {
	// Like SurfaceFromDevice, actual chain buffers determine effective chains.
	d := Device{Type: DeviceTypeLight, ProductID: 201, LightType: LightTypeMatrix,
		MatrixProperties: MatrixProperties{Width: 8, Height: 16, ChainLength: 1,
			ChainZones: [][]packets.LightHsbk{make([]packets.LightHsbk, 128), make([]packets.LightHsbk, 64)}}}
	if count, known := VisibleZoneCount(d); !known || count != 180 {
		t.Fatalf("count=%d known=%v", count, known)
	}
	for _, p := range []MatrixProperties{
		{Width: math.MaxInt, Height: 2, ChainLength: 1},
		{Width: 2, Height: 2, ChainLength: math.MaxInt},
		{Width: 0, Height: 8, ChainLength: 1},
		{Width: 8, Height: -1, ChainLength: 1},
		{Width: 8, Height: 8, ChainLength: 0},
	} {
		d.MatrixProperties = p
		if count, known := VisibleZoneCount(d); known || count != 0 {
			t.Fatalf("invalid geometry got=%d/%v", count, known)
		}
	}
}

func ExampleVisibleZoneCount() {
	d := Device{Type: DeviceTypeLight, LightType: LightTypeMatrix, ProductID: 215,
		MatrixProperties: MatrixProperties{Width: 5, Height: 6, ChainLength: 1, NZones: 30}}
	count, known := VisibleZoneCount(d)
	fmt.Println(count, known)
	// Output:
	// 27 true
}
