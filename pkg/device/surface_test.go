package device

import (
	"reflect"
	"testing"

	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func TestSurfaceFromDeviceSingleZone(t *testing.T) {
	got := SurfaceFromDevice(Device{LightType: LightTypeSingleZone})
	want := Surface{LightType: LightTypeSingleZone, Width: 1, Height: 1, Zones: 1}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("surface = %#v, want %#v", got, want)
	}
}

func TestSurfaceFromDeviceMultiZone(t *testing.T) {
	got := SurfaceFromDevice(Device{
		LightType: LightTypeMultiZone,
		MultizoneProperties: MultizoneProperties{
			Zones: make([]packets.LightHsbk, 8),
		},
	})
	want := Surface{LightType: LightTypeMultiZone, Width: 8, Height: 1, Zones: 8}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("surface = %#v, want %#v", got, want)
	}
}

func TestSurfaceFromDeviceOrdinaryTileChain(t *testing.T) {
	got := SurfaceFromDevice(Device{
		ProductID: 55,
		LightType: LightTypeMatrix,
		MatrixProperties: MatrixProperties{
			Width:             8,
			Height:            8,
			NZones:            64,
			ChainLength:       2,
			ChainOrientations: []Orientation{OrientationRightSideUp, OrientationLeft},
		},
	})

	if got.LightType != LightTypeMatrix || got.Width != 16 || got.Height != 8 || got.Zones != 128 {
		t.Fatalf("surface = %#v, want matrix 16x8 zones=128", got)
	}
	if got.Matrix == nil || len(got.Matrix.Chains) != 2 {
		t.Fatalf("chains = %#v, want 2", got.Matrix)
	}

	first := got.Matrix.Chains[0]
	if first.Index != 0 || first.Bounds != (Rect{X: 0, Y: 0, Width: 8, Height: 8}) || first.SendWidth != 8 || first.Orientation != OrientationRightSideUp {
		t.Fatalf("first chain = %#v", first)
	}
	second := got.Matrix.Chains[1]
	if second.Index != 1 || second.Bounds != (Rect{X: 8, Y: 0, Width: 8, Height: 8}) || second.SendWidth != 8 || second.Orientation != OrientationLeft {
		t.Fatalf("second chain = %#v", second)
	}
	assertRectRows(t, first.Rows, 8, 8)
}

func TestSurfaceFromDeviceUsesChainZonesForChainCount(t *testing.T) {
	got := SurfaceFromDevice(Device{
		LightType: LightTypeMatrix,
		MatrixProperties: MatrixProperties{
			Width:      2,
			Height:     2,
			ChainZones: [][]packets.LightHsbk{make([]packets.LightHsbk, 4), make([]packets.LightHsbk, 4), make([]packets.LightHsbk, 4)},
		},
	})

	if got.Width != 6 || got.Height != 2 || got.Zones != 12 {
		t.Fatalf("surface = %#v, want 6x2 zones=12", got)
	}
	if len(got.Matrix.Chains) != 3 {
		t.Fatalf("chains = %d, want 3", len(got.Matrix.Chains))
	}
}

func TestSurfaceFromDeviceUnknownMatrixFallback(t *testing.T) {
	got := SurfaceFromDevice(Device{
		LightType: LightTypeMatrix,
		MatrixProperties: MatrixProperties{
			Width:       4,
			Height:      2,
			NZones:      8,
			ChainLength: 2,
		},
	})

	if got.Width != 8 || got.Height != 2 || got.Zones != 16 {
		t.Fatalf("surface = %#v, want rectangular 8x2 zones=16", got)
	}
	for _, chain := range got.Matrix.Chains {
		if chain.SendWidth != 4 {
			t.Fatalf("send width = %d, want 4", chain.SendWidth)
		}
		assertRectRows(t, chain.Rows, 4, 2)
	}
}

func TestSurfaceFromDeviceCandleLayout(t *testing.T) {
	got := SurfaceFromDevice(Device{
		ProductID: 57,
		LightType: LightTypeMatrix,
		MatrixProperties: MatrixProperties{
			Width:      5,
			Height:     11,
			NZones:     55,
			ChainZones: [][]packets.LightHsbk{make([]packets.LightHsbk, 55)},
		},
	})

	chain := got.Matrix.Chains[0]
	if chain.Bounds != (Rect{X: 0, Y: 0, Width: 6, Height: 11}) {
		t.Fatalf("bounds = %#v, want 6x11 including row offset", chain.Bounds)
	}
	if chain.SendWidth != 5 {
		t.Fatalf("send width = %d, want 5", chain.SendWidth)
	}
	if len(chain.Rows) != 11 {
		t.Fatalf("rows = %d, want 11", len(chain.Rows))
	}
	if chain.Rows[0].Offset != 1 {
		t.Fatalf("first row offset = %d, want 1", chain.Rows[0].Offset)
	}
	if !reflect.DeepEqual(chain.Rows[0].HiddenCols, []int{2, 3, 4}) {
		t.Fatalf("first row hidden columns = %#v, want [2 3 4]", chain.Rows[0].HiddenCols)
	}
	for i, row := range chain.Rows[1:] {
		if len(row.HiddenCols) != 0 {
			t.Fatalf("row %d hidden columns = %#v, want none", i+1, row.HiddenCols)
		}
	}
}

func TestSurfaceFromDeviceCeilingLayout(t *testing.T) {
	got := SurfaceFromDevice(Device{
		ProductID: 265,
		LightType: LightTypeMatrix,
		MatrixProperties: MatrixProperties{
			Width:      8,
			Height:     8,
			NZones:     64,
			ChainZones: [][]packets.LightHsbk{make([]packets.LightHsbk, 64)},
		},
	})

	rows := got.Matrix.Chains[0].Rows
	want := []int{0, 1, 6, 7}
	if !reflect.DeepEqual(rows[0].HiddenCols, want) {
		t.Fatalf("first row hidden columns = %#v, want %#v", rows[0].HiddenCols, want)
	}
	if !reflect.DeepEqual(rows[7].HiddenCols, want) {
		t.Fatalf("last row hidden columns = %#v, want %#v", rows[7].HiddenCols, want)
	}
}

func TestSurfaceFromDeviceCeilingCapsuleLayout(t *testing.T) {
	got := SurfaceFromDevice(Device{
		ProductID: 201,
		LightType: LightTypeMatrix,
		MatrixProperties: MatrixProperties{
			Width:      8,
			Height:     16,
			NZones:     128,
			ChainZones: [][]packets.LightHsbk{make([]packets.LightHsbk, 128)},
		},
	})

	chain := got.Matrix.Chains[0]
	if chain.SendWidth != 8 {
		t.Fatalf("send width = %d, want original device width 8", chain.SendWidth)
	}
	if chain.Bounds != (Rect{X: 0, Y: 0, Width: 16, Height: 8}) {
		t.Fatalf("bounds = %#v, want 16x8", chain.Bounds)
	}
	if len(chain.Rows) != 8 || chain.Rows[0].Cols != 16 {
		t.Fatalf("rows = %#v, want 8 rows of 16 columns", chain.Rows)
	}
	if !reflect.DeepEqual(chain.Rows[0].HiddenCols, []int{0, 1, 14, 15}) {
		t.Fatalf("first row hidden columns = %#v, want [0 1 14 15]", chain.Rows[0].HiddenCols)
	}
}

func TestSurfaceFromDeviceLunaLayout(t *testing.T) {
	got := SurfaceFromDevice(Device{
		ProductID: 219,
		LightType: LightTypeMatrix,
		MatrixProperties: MatrixProperties{
			Width:      7,
			Height:     5,
			NZones:     35,
			ChainZones: [][]packets.LightHsbk{make([]packets.LightHsbk, 35)},
		},
	})

	rows := got.Matrix.Chains[0].Rows
	if !reflect.DeepEqual(rows[0].HiddenCols, []int{0, 6}) {
		t.Fatalf("first row hidden columns = %#v, want [0 6]", rows[0].HiddenCols)
	}
	if !reflect.DeepEqual(rows[4].HiddenCols, []int{0, 6}) {
		t.Fatalf("last row hidden columns = %#v, want [0 6]", rows[4].HiddenCols)
	}
}

func TestSurfaceFromDeviceClonesReturnedSlices(t *testing.T) {
	dev := Device{
		ProductID: 265,
		LightType: LightTypeMatrix,
		MatrixProperties: MatrixProperties{
			Width:             8,
			Height:            8,
			NZones:            64,
			ChainOrientations: []Orientation{OrientationLeft},
			ChainZones:        [][]packets.LightHsbk{make([]packets.LightHsbk, 64)},
		},
	}
	originalZones := append([]packets.LightHsbk(nil), dev.MatrixProperties.ChainZones[0]...)
	originalOrientations := append([]Orientation(nil), dev.MatrixProperties.ChainOrientations...)

	surface := SurfaceFromDevice(dev)
	surface.Matrix.Chains[0].Rows[0].HiddenCols[0] = 99
	surface.Matrix.Chains[0].Rows[0].Offset = 99
	surface.Matrix.Chains[0].Orientation = OrientationRight

	next := SurfaceFromDevice(dev)
	if !reflect.DeepEqual(next.Matrix.Chains[0].Rows[0].HiddenCols, []int{0, 1, 6, 7}) {
		t.Fatalf("next hidden columns = %#v, want fresh clone", next.Matrix.Chains[0].Rows[0].HiddenCols)
	}
	if next.Matrix.Chains[0].Rows[0].Offset != 0 {
		t.Fatalf("next offset = %d, want 0", next.Matrix.Chains[0].Rows[0].Offset)
	}
	if next.Matrix.Chains[0].Orientation != OrientationLeft {
		t.Fatalf("next orientation = %d, want original left", next.Matrix.Chains[0].Orientation)
	}
	if !reflect.DeepEqual(dev.MatrixProperties.ChainZones[0], originalZones) {
		t.Fatal("SurfaceFromDevice mutated ChainZones")
	}
	if !reflect.DeepEqual(dev.MatrixProperties.ChainOrientations, originalOrientations) {
		t.Fatal("SurfaceFromDevice mutated ChainOrientations")
	}
}

func assertRectRows(t *testing.T, rows []MatrixRow, width, height int) {
	t.Helper()
	if len(rows) != height {
		t.Fatalf("rows = %d, want %d", len(rows), height)
	}
	for i, row := range rows {
		if row.Cols != width || row.Offset != 0 || len(row.HiddenCols) != 0 {
			t.Fatalf("row %d = %#v, want plain width %d", i, row, width)
		}
	}
}
