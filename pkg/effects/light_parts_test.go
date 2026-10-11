package effects

import (
	"math"
	"reflect"
	"testing"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func compositionDevice() device.Device {
	d := device.Device{Type: device.DeviceTypeLight, ProductID: 265, LightType: device.LightTypeMatrix, MatrixProperties: device.MatrixProperties{Width: 8, Height: 8, ChainLength: 1, ChainZones: [][]packets.LightHsbk{make([]packets.LightHsbk, 64)}}}
	for i := range 64 {
		d.MatrixProperties.ChainZones[0][i] = packets.LightHsbk{Hue: uint16(i), Saturation: 12345, Brightness: uint16(i * 511), Kelvin: 3500}
	}
	return d
}

func TestComposePartFrameNoAliasingAndNoOp(t *testing.T) {
	d := compositionDevice()
	before := d.Clone()
	got, err := ComposeLightPartFrame(d, nil, nil, false)
	if err != nil || !reflect.DeepEqual(got, before.MatrixProperties.ChainZones[0]) {
		t.Fatal(err)
	}
	got[0].Brightness = 65535
	if !reflect.DeepEqual(d, before) {
		t.Fatal("output aliases device")
	}
	frame := NewFrame(8, 8, 0, Color{Hue: 220, Saturation: 80, Brightness: 50, Kelvin: 3500})
	up := Color{Hue: 30, Saturation: 100, Brightness: 0, Kelvin: 3500}
	got, err = ComposeLightPartFrame(d, &frame, &up, false)
	mask, _ := LightPartPhysicalMask(d, device.LightPartMain)
	index := 0
	for i, c := range mask {
		if c.Brightness != 0 {
			index = i
			break
		}
	}
	if err != nil || got[63] != up.ToDeviceColor() || got[index] != frame.Colors[0].ToDeviceColor() {
		t.Fatal("frame composition", err)
	}
	frame.Colors[0].Hue = 100
	up.Hue = 100
	if got[index].Hue != device.ConvertExternalToDeviceValue(220, 360) || !reflect.DeepEqual(d, before) {
		t.Fatal("input mutation/aliasing")
	}
}

func TestComposePartFrameRejectsInvalidInput(t *testing.T) {
	d := compositionDevice()
	if _, err := LightPartPhysicalMask(d, device.LightPartAll); err == nil {
		t.Fatal("invalid mask accepted")
	}
	for _, frame := range []Frame{
		NewFrame(7, 8, 0, BlankColor()),
		{Width: 8, Height: 8, Colors: []Color{BlankColor()}},
		NewFrame(8, 8, 0, Color{Hue: math.NaN(), Kelvin: 3500}),
		NewFrame(8, 8, 0, Color{Brightness: 101, Kelvin: 3500}),
	} {
		if _, err := ComposeLightPartFrame(d, &frame, nil, false); err == nil {
			t.Fatal("invalid frame accepted")
		}
	}
	badUp := Color{Brightness: math.Inf(1), Kelvin: 3500}
	if _, err := ComposeLightPartFrame(d, nil, &badUp, false); err == nil {
		t.Fatal("invalid uplight accepted")
	}
	d.MatrixProperties.ChainZones[0] = nil
	if _, err := ComposeLightPartFrame(d, nil, nil, false); err == nil {
		t.Fatal("missing initial state accepted")
	}
}
