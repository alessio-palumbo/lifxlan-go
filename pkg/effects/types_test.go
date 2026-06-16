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
