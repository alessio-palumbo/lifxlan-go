package effects

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func TestPhysicalColorStateAdaptsSingleAndMultizoneSurfaces(t *testing.T) {
	tests := []struct {
		name    string
		surface device.Surface
		start   int
		update  []packets.LightHsbk
		want    []packets.LightHsbk
	}{
		{
			name:    "single zone",
			surface: device.SurfaceFromDevice(device.Device{LightType: device.LightTypeSingleZone}),
			update:  []packets.LightHsbk{physicalTestColor(1)},
			want:    []packets.LightHsbk{physicalTestColor(1)},
		},
		{
			name: "linear multizone",
			surface: device.Surface{
				LightType: device.LightTypeMultiZone,
				Width:     4,
				Height:    1,
				Zones:     4,
			},
			start:  1,
			update: []packets.LightHsbk{physicalTestColor(2), physicalTestColor(3), physicalTestColor(4), physicalTestColor(5)},
			want: []packets.LightHsbk{
				BlankColor().ToDeviceColor(),
				physicalTestColor(2),
				physicalTestColor(3),
				physicalTestColor(4),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := NewPhysicalColorState(tt.surface)
			if err := state.MergeZoneColors(tt.start, tt.update); err != nil {
				t.Fatal(err)
			}
			frame, err := AdaptPhysicalColorStateToFrame(state, tt.surface, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			assertFramePhysicalColors(t, frame, tt.want)
			if frame.Duration != time.Second {
				t.Fatalf("duration = %s, want 1s", frame.Duration)
			}
		})
	}
}

func TestPhysicalColorStateAdaptsEveryMatrixOrientation(t *testing.T) {
	orientations := []device.Orientation{
		device.OrientationRightSideUp,
		device.OrientationUpsideDown,
		device.OrientationFaceUp,
		device.OrientationFaceDown,
		device.OrientationLeft,
		device.OrientationRight,
	}
	logical := physicalTestColors(64, 10)

	for _, orientation := range orientations {
		t.Run(physicalOrientationName(orientation), func(t *testing.T) {
			surface := physicalMatrixSurface(8, 8, orientation)
			state := NewPhysicalColorState(surface)
			state.MatrixChains[0] = device.LogicalMatrixColorsToPhysical(8, 8, orientation, logical)

			frame, err := AdaptPhysicalColorStateToFrame(state, surface, 0)
			if err != nil {
				t.Fatal(err)
			}
			assertFramePhysicalColors(t, frame, logical)
		})
	}
}

func TestPhysicalColorStateUsesMatrixChainBoundsOffsetsAndOrientation(t *testing.T) {
	surface := device.Surface{
		LightType: device.LightTypeMatrix,
		Width:     7,
		Height:    4,
		Zones:     8,
		Matrix: &device.MatrixSurface{Chains: []device.MatrixChain{
			{
				Index:       0,
				Bounds:      device.Rect{X: 1, Y: 1, Width: 2, Height: 2},
				SendWidth:   2,
				Rows:        []device.MatrixRow{{Cols: 2}, {Cols: 2}},
				Orientation: device.OrientationRightSideUp,
			},
			{
				Index:       1,
				Bounds:      device.Rect{X: 4, Y: 0, Width: 2, Height: 2},
				SendWidth:   2,
				Rows:        []device.MatrixRow{{Cols: 2}, {Cols: 2}},
				Orientation: device.OrientationLeft,
			},
		}},
	}
	first := physicalTestColors(4, 1)
	second := physicalTestColors(4, 20)
	state := NewPhysicalColorState(surface)
	state.MatrixChains[0] = first
	state.MatrixChains[1] = device.LogicalMatrixColorsToPhysical(2, 2, device.OrientationLeft, second)

	frame, err := AdaptPhysicalColorStateToFrame(state, surface, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := filledDeviceColors(7*4, BlankColor().ToDeviceColor())
	setPhysicalColor(want, 7, 1, 1, first[0])
	setPhysicalColor(want, 7, 2, 1, first[1])
	setPhysicalColor(want, 7, 1, 2, first[2])
	setPhysicalColor(want, 7, 2, 2, first[3])
	setPhysicalColor(want, 7, 4, 0, second[0])
	setPhysicalColor(want, 7, 5, 0, second[1])
	setPhysicalColor(want, 7, 4, 1, second[2])
	setPhysicalColor(want, 7, 5, 1, second[3])
	assertFramePhysicalColors(t, frame, want)
}

func TestPhysicalColorStateOmitsHiddenCellsAndKeepsRowOffsets(t *testing.T) {
	surface := device.Surface{
		LightType: device.LightTypeMatrix,
		Width:     4,
		Height:    2,
		Zones:     6,
		Matrix: &device.MatrixSurface{Chains: []device.MatrixChain{{
			Index:     0,
			Bounds:    device.Rect{Width: 4, Height: 2},
			SendWidth: 3,
			Rows: []device.MatrixRow{
				{Cols: 3, Offset: 1, HiddenCols: []int{1}},
				{Cols: 3},
			},
		}}},
	}
	physical := physicalTestColors(6, 1)
	state := PhysicalColorState{MatrixChains: [][]packets.LightHsbk{physical}}

	frame, err := AdaptPhysicalColorStateToFrame(state, surface, 0)
	if err != nil {
		t.Fatal(err)
	}
	blank := BlankColor().ToDeviceColor()
	want := []packets.LightHsbk{
		blank, physical[0], blank, physical[2],
		physical[3], physical[4], physical[5], blank,
	}
	assertFramePhysicalColors(t, frame, want)
}

func TestMergeMatrixColorsAppliesPartialRegionsAndCrops(t *testing.T) {
	surface := physicalMatrixSurface(8, 8, device.OrientationRightSideUp)
	state := NewPhysicalColorState(surface)
	partial := physicalTestColors(4, 1)

	if err := state.MergeMatrixColors(surface, 0, 2, 1, 2, partial); err != nil {
		t.Fatal(err)
	}
	if err := state.MergeMatrixColors(surface, 0, 0, 7, 8, physicalTestColors(16, 20)); err != nil {
		t.Fatal(err)
	}

	want := filledDeviceColors(64, BlankColor().ToDeviceColor())
	want[10], want[11], want[18], want[19] = partial[0], partial[1], partial[2], partial[3]
	copy(want[56:64], physicalTestColors(8, 20))
	if !reflect.DeepEqual(state.MatrixChains[0], want) {
		t.Fatalf("physical matrix = %#v, want %#v", state.MatrixChains[0], want)
	}

	partial[0] = physicalTestColor(99)
	if state.MatrixChains[0][10] == partial[0] {
		t.Fatal("merged matrix state aliases update colors")
	}
}

func TestAdaptPhysicalColorStateCropsAndPads(t *testing.T) {
	blank := BlankColor().ToDeviceColor()
	tests := []struct {
		name    string
		surface device.Surface
		state   PhysicalColorState
		want    []packets.LightHsbk
	}{
		{
			name:    "multizone padding",
			surface: device.Surface{LightType: device.LightTypeMultiZone, Width: 4, Height: 1, Zones: 4},
			state:   PhysicalColorState{Zones: physicalTestColors(2, 1)},
			want:    append(physicalTestColors(2, 1), blank, blank),
		},
		{
			name:    "multizone cropping",
			surface: device.Surface{LightType: device.LightTypeMultiZone, Width: 2, Height: 1, Zones: 2},
			state:   PhysicalColorState{Zones: physicalTestColors(4, 1)},
			want:    physicalTestColors(2, 1),
		},
		{
			name:    "matrix padding",
			surface: physicalMatrixSurface(2, 2, device.OrientationRightSideUp),
			state:   PhysicalColorState{MatrixChains: [][]packets.LightHsbk{physicalTestColors(2, 1)}},
			want:    append(physicalTestColors(2, 1), blank, blank),
		},
		{
			name:    "matrix cropping",
			surface: physicalMatrixSurface(2, 2, device.OrientationRightSideUp),
			state:   PhysicalColorState{MatrixChains: [][]packets.LightHsbk{physicalTestColors(6, 1)}},
			want:    physicalTestColors(4, 1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame, err := AdaptPhysicalColorStateToFrame(tt.state, tt.surface, 0)
			if err != nil {
				t.Fatal(err)
			}
			assertFramePhysicalColors(t, frame, tt.want)
		})
	}
}

func TestLogicalFrameAndPhysicalStateRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		surface device.Surface
	}{
		{
			name:    "single zone",
			surface: device.SurfaceFromDevice(device.Device{LightType: device.LightTypeSingleZone}),
		},
		{
			name:    "multizone",
			surface: device.Surface{LightType: device.LightTypeMultiZone, Width: 4, Height: 1, Zones: 4},
		},
		{
			name: "matrix chain",
			surface: device.Surface{
				LightType: device.LightTypeMatrix,
				Width:     4,
				Height:    2,
				Zones:     8,
				Matrix: &device.MatrixSurface{Chains: []device.MatrixChain{
					{Index: 0, Bounds: device.Rect{Width: 2, Height: 2}, SendWidth: 2, Rows: []device.MatrixRow{{Cols: 2}, {Cols: 2}}, Orientation: device.OrientationRight},
					{Index: 1, Bounds: device.Rect{X: 2, Width: 2, Height: 2}, SendWidth: 2, Rows: []device.MatrixRow{{Cols: 2}, {Cols: 2}}, Orientation: device.OrientationUpsideDown},
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logicalRaw := physicalTestColors(tt.surface.Width*tt.surface.Height, 1)
			logical := exactFrameFromPhysical(tt.surface.Width, tt.surface.Height, logicalRaw)
			adapted, err := AdaptFrameToSurface(logical, tt.surface, AdaptOptions{})
			if err != nil {
				t.Fatal(err)
			}

			state := NewPhysicalColorState(tt.surface)
			if tt.surface.LightType == device.LightTypeMatrix {
				for _, frame := range adapted {
					colors := frameColorsToPhysical(frame.Colors)
					state.MatrixChains[frame.ChainIndex] = device.LogicalMatrixColorsToPhysical(frame.SendWidth, frame.Height, frame.Orientation, colors)
				}
			} else {
				state.Zones = frameColorsToPhysical(adapted[0].Colors)
			}

			roundTripped, err := AdaptPhysicalColorStateToFrame(state, tt.surface, logical.Duration)
			if err != nil {
				t.Fatal(err)
			}
			assertFramePhysicalColors(t, roundTripped, logicalRaw)

			readapted, err := AdaptFrameToSurface(roundTripped, tt.surface, AdaptOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if tt.surface.LightType == device.LightTypeMatrix {
				for _, frame := range readapted {
					got := device.LogicalMatrixColorsToPhysical(frame.SendWidth, frame.Height, frame.Orientation, frameColorsToPhysical(frame.Colors))
					if !reflect.DeepEqual(got, state.MatrixChains[frame.ChainIndex]) {
						t.Fatalf("physical chain %d = %#v, want %#v", frame.ChainIndex, got, state.MatrixChains[frame.ChainIndex])
					}
				}
			} else if got := frameColorsToPhysical(readapted[0].Colors); !reflect.DeepEqual(got, state.Zones) {
				t.Fatalf("physical zones = %#v, want %#v", got, state.Zones)
			}
		})
	}
}

func TestPhysicalColorUpdateValidation(t *testing.T) {
	zoneState := NewPhysicalColorState(device.Surface{LightType: device.LightTypeMultiZone, Width: 2, Height: 1, Zones: 2})
	if err := zoneState.MergeZoneColors(-1, physicalTestColors(1, 1)); !errors.Is(err, ErrInvalidPhysicalUpdate) {
		t.Fatalf("zone error = %v, want %v", err, ErrInvalidPhysicalUpdate)
	}

	surface := physicalMatrixSurface(2, 2, device.OrientationRightSideUp)
	matrixState := NewPhysicalColorState(surface)
	tests := []struct {
		name       string
		chain, x   int
		y, width   int
		wantUpdate bool
	}{
		{name: "negative x", x: -1, width: 1, wantUpdate: true},
		{name: "negative y", y: -1, width: 1, wantUpdate: true},
		{name: "zero width", width: 0, wantUpdate: true},
		{name: "missing chain", chain: 1, width: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := matrixState.MergeMatrixColors(surface, tt.chain, tt.x, tt.y, tt.width, physicalTestColors(1, 1))
			want := ErrInvalidPhysicalState
			if tt.wantUpdate {
				want = ErrInvalidPhysicalUpdate
			}
			if !errors.Is(err, want) {
				t.Fatalf("error = %v, want %v", err, want)
			}
		})
	}
}

func physicalMatrixSurface(width, height int, orientation device.Orientation) device.Surface {
	rows := make([]device.MatrixRow, height)
	for i := range rows {
		rows[i] = device.MatrixRow{Cols: width}
	}
	return device.Surface{
		LightType: device.LightTypeMatrix,
		Width:     width,
		Height:    height,
		Zones:     width * height,
		Matrix: &device.MatrixSurface{Chains: []device.MatrixChain{{
			Index:       0,
			Bounds:      device.Rect{Width: width, Height: height},
			SendWidth:   width,
			Rows:        rows,
			Orientation: orientation,
		}}},
	}
}

func physicalTestColors(count int, start uint16) []packets.LightHsbk {
	colors := make([]packets.LightHsbk, count)
	for i := range colors {
		colors[i] = physicalTestColor(start + uint16(i))
	}
	return colors
}

func physicalTestColor(value uint16) packets.LightHsbk {
	return packets.LightHsbk{
		Hue:        value*997 + 1,
		Saturation: value*613 + 2,
		Brightness: value*379 + 3,
		Kelvin:     2500 + value,
	}
}

func exactFrameFromPhysical(width, height int, colors []packets.LightHsbk) Frame {
	frame := Frame{Colors: make([]Color, len(colors)), Width: width, Height: height, Duration: time.Second}
	for i, color := range colors {
		frame.Colors[i] = exactDeviceColor(color)
	}
	return frame
}

func frameColorsToPhysical(colors []Color) []packets.LightHsbk {
	physical := make([]packets.LightHsbk, len(colors))
	for i, color := range colors {
		physical[i] = color.ToDeviceColor()
	}
	return physical
}

func assertFramePhysicalColors(t *testing.T, frame Frame, want []packets.LightHsbk) {
	t.Helper()
	got := frameColorsToPhysical(frame.Colors)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("frame physical colors = %#v, want %#v", got, want)
	}
}

func setPhysicalColor(colors []packets.LightHsbk, width, x, y int, color packets.LightHsbk) {
	colors[y*width+x] = color
}

func physicalOrientationName(orientation device.Orientation) string {
	switch orientation {
	case device.OrientationRightSideUp:
		return "right_side_up"
	case device.OrientationUpsideDown:
		return "upside_down"
	case device.OrientationFaceUp:
		return "face_up"
	case device.OrientationFaceDown:
		return "face_down"
	case device.OrientationLeft:
		return "left"
	case device.OrientationRight:
		return "right"
	default:
		return "unknown"
	}
}
