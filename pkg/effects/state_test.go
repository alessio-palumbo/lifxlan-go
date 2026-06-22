package effects

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func TestFrameFromDeviceStateSingleZone(t *testing.T) {
	dev := device.Device{Color: color(10)}

	got, err := FrameFromDeviceState(dev, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	want := Frame{Colors: []Color{color(10)}, Width: 1, Height: 1, Duration: time.Second}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("frame = %#v, want %#v", got, want)
	}
}

func TestFrameFromDeviceStateMultizone(t *testing.T) {
	dev := device.Device{
		LightType: device.LightTypeMultiZone,
		MultizoneProperties: device.MultizoneProperties{
			Zones: []packets.LightHsbk{color(10).ToDeviceColor(), color(20).ToDeviceColor()},
		},
	}

	got, err := FrameFromDeviceState(dev, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	want := Frame{Colors: []Color{color(10), color(20)}, Width: 2, Height: 1, Duration: time.Second}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("frame = %#v, want %#v", got, want)
	}
}

func TestFrameFromDeviceStateMatrixTileChain(t *testing.T) {
	dev := device.Device{
		LightType: device.LightTypeMatrix,
		MatrixProperties: device.MatrixProperties{
			Width:       2,
			Height:      2,
			ChainLength: 2,
			ChainZones: [][]packets.LightHsbk{
				deviceColors(color(0), color(1), color(2), color(3)),
				deviceColors(color(4), color(5), color(6), color(7)),
			},
		},
	}

	got, err := FrameFromDeviceState(dev, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	want := Frame{
		Colors:   []Color{color(0), color(1), color(4), color(5), color(2), color(3), color(6), color(7)},
		Width:    4,
		Height:   2,
		Duration: time.Second,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("frame = %#v, want %#v", got, want)
	}
}

func TestFrameFromDeviceStateMatrixInvertsOrientation(t *testing.T) {
	dev := device.Device{
		LightType: device.LightTypeMatrix,
		MatrixProperties: device.MatrixProperties{
			Width:             2,
			Height:            2,
			ChainLength:       1,
			ChainOrientations: []device.Orientation{device.OrientationLeft},
			ChainZones: [][]packets.LightHsbk{
				deviceColors(color(2), color(0), color(3), color(1)),
			},
		},
	}

	got, err := FrameFromDeviceState(dev, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	want := Frame{Colors: []Color{color(0), color(1), color(2), color(3)}, Width: 2, Height: 2, Duration: time.Second}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("frame = %#v, want %#v", got, want)
	}
}

func TestFrameFromDeviceStateMatrixRoundTripsAdaptedFrame(t *testing.T) {
	dev := device.Device{
		LightType: device.LightTypeMatrix,
		MatrixProperties: device.MatrixProperties{
			Width:             2,
			Height:            2,
			ChainLength:       2,
			ChainOrientations: []device.Orientation{device.OrientationLeft, device.OrientationRightSideUp},
			ChainZones:        [][]packets.LightHsbk{make([]packets.LightHsbk, 4), make([]packets.LightHsbk, 4)},
		},
	}
	want := Frame{
		Colors:   []Color{color(0), color(1), color(4), color(5), color(2), color(3), color(6), color(7)},
		Width:    4,
		Height:   2,
		Duration: time.Second,
	}

	deviceFrames, err := AdaptFrameToSurface(want, device.SurfaceFromDevice(dev), AdaptOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, frame := range deviceFrames {
		physical := device.ReorientMatrix(frame.SendWidth, frame.Height, frame.Orientation, frameColorsToDevice(frame.Colors))
		dev.MatrixProperties.ChainZones[frame.ChainIndex] = physical
	}

	got, err := FrameFromDeviceState(dev, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("frame = %#v, want %#v", got, want)
	}
}

func TestFrameFromDeviceStateRejectsMissingMatrixState(t *testing.T) {
	dev := device.Device{
		LightType: device.LightTypeMatrix,
		MatrixProperties: device.MatrixProperties{
			Width:       2,
			Height:      2,
			ChainLength: 1,
		},
	}

	_, err := FrameFromDeviceState(dev, time.Second)
	if !errors.Is(err, ErrInvalidDeviceState) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidDeviceState)
	}
}

func deviceColors(colors ...Color) []packets.LightHsbk {
	out := make([]packets.LightHsbk, len(colors))
	for i, color := range colors {
		out[i] = color.ToDeviceColor()
	}
	return out
}

func frameColorsToDevice(colors []Color) []packets.LightHsbk {
	out := make([]packets.LightHsbk, len(colors))
	for i, color := range colors {
		out[i] = color.ToDeviceColor()
	}
	return out
}
