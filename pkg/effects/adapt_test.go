package effects

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func TestAdaptFrameToSurfaceSingleZoneReduction(t *testing.T) {
	frame := Frame{
		Colors: []Color{
			{Hue: 0, Saturation: 100, Brightness: 20, Kelvin: 3000},
			{Hue: 60, Saturation: 50, Brightness: 80, Kelvin: 5000},
		},
		Width:    2,
		Height:   1,
		Duration: time.Second,
	}
	surface := device.SurfaceFromDevice(device.Device{LightType: device.LightTypeSingleZone})

	first, err := AdaptFrameToSurface(frame, surface, AdaptOptions{Reduction: ReductionFirst})
	if err != nil {
		t.Fatal(err)
	}
	if got := first[0].Colors[0]; got != frame.Colors[0] {
		t.Fatalf("first reduction = %#v, want %#v", got, frame.Colors[0])
	}

	average, err := AdaptFrameToSurface(frame, surface, AdaptOptions{Reduction: ReductionAverage})
	if err != nil {
		t.Fatal(err)
	}
	want := Color{Hue: 30, Saturation: 75, Brightness: 50, Kelvin: 4000}
	if got := average[0].Colors[0]; math.Abs(got.Hue-want.Hue) > 1e-9 ||
		got.Saturation != want.Saturation ||
		got.Brightness != want.Brightness ||
		got.Kelvin != want.Kelvin {
		t.Fatalf("average reduction = %#v, want %#v", got, want)
	}
}

func TestAdaptFrameToSurfaceAverageReductionUsesCircularHue(t *testing.T) {
	frame := Frame{
		Colors: []Color{
			{Hue: 359, Saturation: 100, Brightness: 20, Kelvin: 3000},
			{Hue: 1, Saturation: 50, Brightness: 80, Kelvin: 5000},
		},
		Width:  2,
		Height: 1,
	}
	surface := device.SurfaceFromDevice(device.Device{LightType: device.LightTypeSingleZone})

	got, err := AdaptFrameToSurface(frame, surface, AdaptOptions{Reduction: ReductionAverage})
	if err != nil {
		t.Fatal(err)
	}
	hue := got[0].Colors[0].Hue
	if math.Abs(hue) > 1e-9 && math.Abs(hue-360) > 1e-9 {
		t.Fatalf("average hue = %f, want near 0", hue)
	}
}

func TestAdaptFrameToSurfaceMultiZoneScalesStrip(t *testing.T) {
	frame := indexedFrame(4, 1, time.Second)
	surface := device.Surface{
		LightType: device.LightTypeMultiZone,
		Width:     2,
		Height:    1,
		Zones:     2,
	}

	got, err := AdaptFrameToSurface(frame, surface, AdaptOptions{})
	if err != nil {
		t.Fatal(err)
	}

	want := []Color{frame.Colors[0], frame.Colors[2]}
	if len(got) != 1 || got[0].SendWidth != 2 || got[0].Height != 1 || !reflect.DeepEqual(got[0].Colors, want) {
		t.Fatalf("adapted frame = %#v, want colors %#v", got, want)
	}
}

func TestAdaptFrameToSurfaceOrdinaryTileChain(t *testing.T) {
	frame := indexedFrame(16, 8, time.Second)
	surface := device.SurfaceFromDevice(device.Device{
		ProductID: 55,
		LightType: device.LightTypeMatrix,
		MatrixProperties: device.MatrixProperties{
			Width:       8,
			Height:      8,
			NZones:      64,
			ChainLength: 2,
			ChainOrientations: []device.Orientation{
				device.OrientationRightSideUp,
				device.OrientationLeft,
			},
		},
	})

	got, err := AdaptFrameToSurface(frame, surface, AdaptOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 2 {
		t.Fatalf("frames = %d, want 2", len(got))
	}
	if got[0].ChainIndex != 0 || got[0].SendWidth != 8 || got[0].Height != 8 || got[0].Orientation != device.OrientationRightSideUp {
		t.Fatalf("first frame metadata = %#v", got[0])
	}
	if got[1].ChainIndex != 1 || got[1].SendWidth != 8 || got[1].Height != 8 || got[1].Orientation != device.OrientationLeft {
		t.Fatalf("second frame metadata = %#v", got[1])
	}
	if got[0].Colors[0] != frame.Colors[0] || got[0].Colors[7] != frame.Colors[7] || got[0].Colors[8] != frame.Colors[16] {
		t.Fatalf("first chain colors = %#v", got[0].Colors[:9])
	}
	if got[1].Colors[0] != frame.Colors[8] || got[1].Colors[7] != frame.Colors[15] || got[1].Colors[8] != frame.Colors[24] {
		t.Fatalf("second chain colors = %#v", got[1].Colors[:9])
	}
}

func TestAdaptFrameToSurfaceCandleOffsetAndHiddenCols(t *testing.T) {
	frame := indexedFrame(6, 11, time.Second)
	surface := device.SurfaceFromDevice(device.Device{
		ProductID: 57,
		LightType: device.LightTypeMatrix,
		MatrixProperties: device.MatrixProperties{
			Width:      5,
			Height:     11,
			NZones:     55,
			ChainZones: [][]packets.LightHsbk{make([]packets.LightHsbk, 55)},
		},
	})

	got, err := AdaptFrameToSurface(frame, surface, AdaptOptions{})
	if err != nil {
		t.Fatal(err)
	}

	colors := got[0].Colors
	if got[0].SendWidth != 5 || got[0].Height != 11 || len(colors) != 55 {
		t.Fatalf("frame metadata = %#v", got[0])
	}
	if colors[0] != frame.Colors[1] || colors[1] != frame.Colors[2] {
		t.Fatalf("offset colors = %#v, want source x=1 and x=2", colors[:2])
	}
	for _, index := range []int{2, 3, 4} {
		if colors[index] != blankColor {
			t.Fatalf("hidden slot %d = %#v, want blank color", index, colors[index])
		}
	}
	if colors[5] != frame.Colors[6] {
		t.Fatalf("second row first color = %#v, want %#v", colors[5], frame.Colors[6])
	}
}

func TestAdaptFrameToSurfaceCeilingCapsuleKeepsSendWidth(t *testing.T) {
	frame := indexedFrame(16, 8, time.Second)
	surface := device.SurfaceFromDevice(device.Device{
		ProductID: 201,
		LightType: device.LightTypeMatrix,
		MatrixProperties: device.MatrixProperties{
			Width:      8,
			Height:     16,
			NZones:     128,
			ChainZones: [][]packets.LightHsbk{make([]packets.LightHsbk, 128)},
		},
	})

	got, err := AdaptFrameToSurface(frame, surface, AdaptOptions{})
	if err != nil {
		t.Fatal(err)
	}

	colors := got[0].Colors
	if got[0].SendWidth != 8 || got[0].Height != 16 || len(colors) != 128 {
		t.Fatalf("frame metadata = %#v", got[0])
	}
	for _, index := range []int{0, 1, 14, 15, 112, 113, 126, 127} {
		if colors[index] != blankColor {
			t.Fatalf("hidden slot %d = %#v, want blank color", index, colors[index])
		}
	}
	if colors[2] != frame.Colors[2] || colors[16] != frame.Colors[16] {
		t.Fatalf("capsule mapped colors = slot2 %#v slot16 %#v", colors[2], colors[16])
	}
}

func TestAdaptFrameToSurfacePadsAndCropsMatrixCells(t *testing.T) {
	surface := device.SurfaceFromDevice(device.Device{
		LightType: device.LightTypeMatrix,
		MatrixProperties: device.MatrixProperties{
			Width:       2,
			Height:      1,
			NZones:      2,
			ChainLength: 1,
		},
	})

	paddedFrame := indexedFrame(1, 1, time.Second)
	padded, err := AdaptFrameToSurface(paddedFrame, surface, AdaptOptions{})
	if err != nil {
		t.Fatal(err)
	}
	wantPadded := []Color{paddedFrame.Colors[0], blankColor}
	if !reflect.DeepEqual(padded[0].Colors, wantPadded) {
		t.Fatalf("padded colors = %#v, want %#v", padded[0].Colors, wantPadded)
	}

	cropFrame := indexedFrame(3, 1, time.Second)
	cropped, err := AdaptFrameToSurface(cropFrame, surface, AdaptOptions{})
	if err != nil {
		t.Fatal(err)
	}
	wantCropped := cropFrame.Colors[:2]
	if !reflect.DeepEqual(cropped[0].Colors, wantCropped) {
		t.Fatalf("cropped colors = %#v, want %#v", cropped[0].Colors, wantCropped)
	}
}

func TestAdaptFrameToSurfaceRejectsInvalidFrames(t *testing.T) {
	surface := device.SurfaceFromDevice(device.Device{LightType: device.LightTypeSingleZone})

	testCases := map[string]struct {
		frame Frame
		want  error
	}{
		"empty": {
			frame: Frame{Width: 1, Height: 1},
			want:  ErrEmptyFrame,
		},
		"zero width": {
			frame: Frame{Colors: []Color{{}}, Height: 1},
			want:  ErrInvalidFrame,
		},
		"too few colors": {
			frame: Frame{Colors: []Color{{}}, Width: 2, Height: 1},
			want:  ErrInvalidFrame,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if _, err := AdaptFrameToSurface(tc.frame, surface, AdaptOptions{}); !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func indexedFrame(width, height int, duration time.Duration) Frame {
	colors := make([]Color, width*height)
	for i := range colors {
		colors[i] = Color{
			Hue:        float64(i),
			Saturation: 100,
			Brightness: float64(i + 1),
			Kelvin:     3500,
		}
	}
	return Frame{
		Colors:   colors,
		Width:    width,
		Height:   height,
		Duration: duration,
	}
}
