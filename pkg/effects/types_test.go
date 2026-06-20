package effects

import (
	"reflect"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func TestColorAliasesDeviceColor(t *testing.T) {
	deviceColor := device.Color{Hue: 120, Saturation: 80, Brightness: 60, Kelvin: 3500}

	frame := Frame{
		Colors: []Color{deviceColor},
	}

	if got := frame.Colors[0]; got != deviceColor {
		t.Fatalf("frame color = %#v, want %#v", got, deviceColor)
	}
}

func TestCapabilitiesFromDeviceSingleZone(t *testing.T) {
	got := CapabilitiesFromDevice(device.Device{
		LightType: device.LightTypeSingleZone,
		ColorProperties: device.ColorProperties{
			HasColor:         true,
			TemperatureRange: device.TemperatureRange{Min: 1500, Max: 9000},
		},
	})

	want := Capabilities{
		LightType:        device.LightTypeSingleZone,
		Zones:            1,
		Width:            1,
		Height:           1,
		HasColor:         true,
		TemperatureRange: device.TemperatureRange{Min: 1500, Max: 9000},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("capabilities = %#v, want %#v", got, want)
	}
}

func TestCapabilitiesFromDeviceMultiZone(t *testing.T) {
	got := CapabilitiesFromDevice(device.Device{
		LightType: device.LightTypeMultiZone,
		MultizoneProperties: device.MultizoneProperties{
			Zones: make([]packets.LightHsbk, 8),
		},
		ColorProperties: device.ColorProperties{
			HasColor:         true,
			TemperatureRange: device.TemperatureRange{Min: 2500, Max: 9000},
		},
	})

	want := Capabilities{
		LightType:        device.LightTypeMultiZone,
		Zones:            8,
		Width:            8,
		Height:           1,
		HasColor:         true,
		TemperatureRange: device.TemperatureRange{Min: 2500, Max: 9000},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("capabilities = %#v, want %#v", got, want)
	}
}

func TestCapabilitiesFromDeviceMatrix(t *testing.T) {
	orientations := []device.Orientation{device.OrientationRightSideUp, device.OrientationLeft}
	got := CapabilitiesFromDevice(device.Device{
		LightType: device.LightTypeMatrix,
		MatrixProperties: device.MatrixProperties{
			Width:             8,
			Height:            8,
			NZones:            64,
			ChainLength:       2,
			ChainOrientations: orientations,
		},
		ColorProperties: device.ColorProperties{
			HasColor:         true,
			TemperatureRange: device.TemperatureRange{Min: 1500, Max: 9000},
		},
	})

	want := Capabilities{
		LightType:         device.LightTypeMatrix,
		Zones:             128,
		Width:             16,
		Height:            8,
		ChainLength:       2,
		ChainOrientations: []device.Orientation{device.OrientationRightSideUp, device.OrientationLeft},
		HasColor:          true,
		TemperatureRange:  device.TemperatureRange{Min: 1500, Max: 9000},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("capabilities = %#v, want %#v", got, want)
	}

	orientations[0] = device.OrientationUpsideDown
	if got.ChainOrientations[0] != device.OrientationRightSideUp {
		t.Fatal("capabilities should copy chain orientations")
	}
}

func TestCapabilitiesFromDeviceMatrixComputesZones(t *testing.T) {
	got := CapabilitiesFromDevice(device.Device{
		LightType: device.LightTypeMatrix,
		MatrixProperties: device.MatrixProperties{
			Width:  16,
			Height: 8,
		},
	})

	if got.Zones != 128 {
		t.Fatalf("zones = %d, want 128", got.Zones)
	}
}

func TestCapabilitiesFromDeviceMatrixUsesSurfaceWidth(t *testing.T) {
	got := CapabilitiesFromDevice(device.Device{
		LightType: device.LightTypeMatrix,
		MatrixProperties: device.MatrixProperties{
			Width:       8,
			Height:      8,
			NZones:      64,
			ChainLength: 2,
		},
	})

	if got.Width != 16 || got.Height != 8 || got.Zones != 128 || got.ChainLength != 2 {
		t.Fatalf("capabilities = %#v, want full chain 16x8 zones=128 chainLength=2", got)
	}
}

func TestFrameHasNoTargetFields(t *testing.T) {
	frameType := reflect.TypeOf(Frame{})
	for _, name := range []string{"Target", "Serial", "Group", "Label", "Address", "DeviceID"} {
		if _, ok := frameType.FieldByName(name); ok {
			t.Fatalf("Frame should not contain target field %q", name)
		}
	}
}

func TestBlankColor(t *testing.T) {
	got := BlankColor()
	if got.Brightness != 0 || got.Kelvin == 0 {
		t.Fatalf("BlankColor = %#v, want off color with valid kelvin", got)
	}
}

func TestFrameSize(t *testing.T) {
	tests := map[string]struct {
		width  int
		height int
		want   int
	}{
		"valid": {
			width:  3,
			height: 2,
			want:   6,
		},
		"zero width": {
			width:  0,
			height: 2,
			want:   0,
		},
		"negative height": {
			width:  2,
			height: -1,
			want:   0,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := FrameSize(tt.width, tt.height); got != tt.want {
				t.Fatalf("FrameSize = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNewFrame(t *testing.T) {
	got := NewFrame(2, 3, time.Second, color(10))
	want := Frame{
		Colors:   []Color{color(10), color(10), color(10), color(10), color(10), color(10)},
		Width:    2,
		Height:   3,
		Duration: time.Second,
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("frame = %#v, want %#v", got, want)
	}
}

func TestValidateFrame(t *testing.T) {
	if err := ValidateFrame(NewFrame(1, 1, time.Second, color(10))); err != nil {
		t.Fatalf("ValidateFrame valid frame error = %v", err)
	}
	if err := ValidateFrame(Frame{Width: 1, Height: 1}); err != ErrEmptyFrame {
		t.Fatalf("ValidateFrame empty error = %v, want %v", err, ErrEmptyFrame)
	}
	if err := ValidateFrame(Frame{Colors: []Color{color(10)}, Width: 2, Height: 1}); err != ErrInvalidFrame {
		t.Fatalf("ValidateFrame invalid error = %v, want %v", err, ErrInvalidFrame)
	}
}

func TestFrameColor(t *testing.T) {
	frame := Frame{
		Colors: []Color{
			color(10), color(20),
			color(30), color(40),
		},
		Width:  2,
		Height: 2,
	}

	got, ok := FrameColor(frame, 1, 1)
	if !ok || got != color(40) {
		t.Fatalf("FrameColor = %#v, %t, want color 40 and true", got, ok)
	}

	for _, coord := range [][2]int{{-1, 0}, {0, -1}, {2, 0}, {0, 2}} {
		if got, ok := FrameColor(frame, coord[0], coord[1]); ok || got != (Color{}) {
			t.Fatalf("FrameColor(%d,%d) = %#v, %t, want zero and false", coord[0], coord[1], got, ok)
		}
	}

	shortFrame := Frame{Colors: []Color{color(10)}, Width: 2, Height: 1}
	if got, ok := FrameColor(shortFrame, 1, 0); ok || got != (Color{}) {
		t.Fatalf("FrameColor short frame = %#v, %t, want zero and false", got, ok)
	}
}

func TestSetFrameColor(t *testing.T) {
	frame := NewFrame(2, 2, time.Second, BlankColor())

	if ok := SetFrameColor(&frame, 1, 0, color(10)); !ok {
		t.Fatal("SetFrameColor valid coordinate returned false")
	}
	if got := frame.Colors[1]; got != color(10) {
		t.Fatalf("set color = %#v, want %#v", got, color(10))
	}

	if ok := SetFrameColor(&frame, 2, 0, color(20)); ok {
		t.Fatal("SetFrameColor out-of-bounds coordinate returned true")
	}
	if ok := SetFrameColor(nil, 0, 0, color(20)); ok {
		t.Fatal("SetFrameColor nil frame returned true")
	}

	shortFrame := Frame{Colors: []Color{color(10)}, Width: 2, Height: 1}
	if ok := SetFrameColor(&shortFrame, 1, 0, color(20)); ok {
		t.Fatal("SetFrameColor short frame returned true")
	}
}

type staticEffect struct {
	nextCalled bool
}

func (e *staticEffect) Next(dt time.Duration) (Frame, bool) {
	e.nextCalled = true
	return Frame{Duration: dt}, true
}

func (e *staticEffect) Reset() {
	e.nextCalled = false
}

func TestEffectInterface(t *testing.T) {
	var effect Effect = &staticEffect{}

	frame, ok := effect.Next(time.Second)
	if !ok {
		t.Fatal("Next returned ok=false")
	}
	if frame.Duration != time.Second {
		t.Fatalf("duration = %s, want %s", frame.Duration, time.Second)
	}
}
