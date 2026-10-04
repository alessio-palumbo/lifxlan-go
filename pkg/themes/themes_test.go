package themes

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/effects"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func testTheme() Theme {
	return Theme{Name: "Evening", Palette: effects.Palette{Base: []effects.Color{{Hue: 350, Saturation: 100, Brightness: 30, Kelvin: 3000}, {Hue: 10, Saturation: 50, Brightness: 70, Kelvin: 5000}}}}
}
func testDevice(id byte) device.Device {
	return device.Device{Serial: device.Serial{0xd0, 0x73, 0xd5, 0, 0, id}, ProductID: 27, Type: device.DeviceTypeLight, LightType: device.LightTypeSingleZone, ColorProperties: device.ColorProperties{HasColor: true}}
}

func TestDeterministicSingleZoneAssignmentAndNoMutation(t *testing.T) {
	theme := testTheme()
	a, b := testDevice(1), testDevice(2)
	a.PoweredOn = false
	b.PoweredOn = true
	devices := []device.Device{b, a}
	before := append([]device.Device(nil), devices...)
	first, err := theme.Plan(devices, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	second, err := theme.Plan([]device.Device{a, b}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) || !reflect.DeepEqual(devices, before) {
		t.Fatal("order-dependent plan or input mutation")
	}
	if first[0].Frame.Colors[0] != theme.Palette.Base[0] || first[1].Frame.Colors[0] != theme.Palette.Base[1] {
		t.Fatal("palette not distributed")
	}
	first[0].Frame.Colors[0].Hue = 100
	if theme.Palette.Base[0].Hue != 350 {
		t.Fatal("frame aliases palette")
	}
}

func TestGradientHueAndBrightnessInterpolation(t *testing.T) {
	d := testDevice(1)
	d.LightType = device.LightTypeMultiZone
	d.MultizoneProperties.Zones = make([]packets.LightHsbk, 5)
	plan, err := testTheme().Plan([]device.Device{d}, 0)
	if err != nil {
		t.Fatal(err)
	}
	colors := plan[0].Frame.Colors
	if colors[0].Hue != 350 || colors[4].Hue != 10 || math.Abs(colors[2].Hue) > 1e-9 || colors[2].Brightness != 50 || colors[2].Kelvin != 4000 {
		t.Fatal(colors)
	}
}

func TestStepsSolidAndVerticalMatrix(t *testing.T) {
	d := testDevice(1)
	d.LightType = device.LightTypeMatrix
	d.Type = device.DeviceTypeHybrid
	d.MatrixProperties = device.MatrixProperties{Width: 8, Height: 8, ChainLength: 1, NZones: 64, ChainZones: [][]packets.LightHsbk{make([]packets.LightHsbk, 64)}}
	theme := testTheme()
	theme.Axis = Vertical
	theme.Layout = Steps
	plan, err := theme.Plan([]device.Device{d}, 0)
	if err != nil {
		t.Fatal(err)
	}
	frame := plan[0].Frame
	if frame.Width != 8 || frame.Height != 8 || frame.Colors[0] != theme.Palette.Base[0] || frame.Colors[7] != frame.Colors[0] || frame.Colors[63] != theme.Palette.Base[1] {
		t.Fatal(frame)
	}
	theme.Layout = Solid
	plan, err = theme.Plan([]device.Device{d}, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range plan[0].Frame.Colors {
		if c != theme.Palette.Base[0] {
			t.Fatal("solid is not uniform")
		}
	}
}

func TestTemperatureOnlyDeviceAndBounds(t *testing.T) {
	d := testDevice(1)
	d.ColorProperties = device.ColorProperties{HasColor: false, TemperatureRange: device.TemperatureRange{Min: 4000, Max: 6500}}
	plan, err := testTheme().Plan([]device.Device{d}, 0)
	if err != nil {
		t.Fatal(err)
	}
	c := plan[0].Frame.Colors[0]
	if c.Saturation != 0 || c.Kelvin != 4000 || c.Brightness != 30 {
		t.Fatal(c)
	}
}

func TestInvalidThemeAndGeometry(t *testing.T) {
	for _, mutate := range []func(*Theme){func(t *Theme) { t.Name = "" }, func(t *Theme) { t.Layout = "bad" }, func(t *Theme) { t.Axis = "bad" }, func(t *Theme) { t.Palette = effects.Palette{} }, func(t *Theme) { t.Palette.Base[0].Hue = math.NaN() }, func(t *Theme) { t.Palette.Base[0].Brightness = 101 }, func(t *Theme) { t.Palette.Base[0].Kelvin = 0 }} {
		theme := testTheme()
		mutate(&theme)
		if err := theme.Validate(); err == nil {
			t.Fatal("invalid theme accepted")
		}
	}
	d := testDevice(1)
	for _, devices := range [][]device.Device{nil, {d, d}, {{}}, {{Serial: d.Serial, ProductID: 27, Type: device.DeviceTypeSwitch}}, {{Serial: d.Serial, ProductID: 27, LightType: device.LightTypeMatrix}}, {{Serial: d.Serial, ProductID: 27, LightType: device.LightTypeMultiZone}}} {
		if _, err := testTheme().Plan(devices, 0); err == nil {
			t.Fatal("invalid selection accepted")
		}
	}
	if _, err := testTheme().Plan([]device.Device{d}, -time.Second); err == nil {
		t.Fatal("negative transition accepted")
	}
	if _, err := testTheme().Plan([]device.Device{d}, time.Duration(math.MaxUint32)*time.Millisecond+time.Millisecond); err == nil {
		t.Fatal("overflowing transition accepted")
	}
}

func TestThemeValidBlackIsPreserved(t *testing.T) {
	theme := testTheme()
	for i := range theme.Palette.Base {
		theme.Palette.Base[i].Brightness = 0
	}
	plan, err := theme.Plan([]device.Device{testDevice(1)}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if plan[0].Frame.Colors[0].Brightness != 0 {
		t.Fatal("valid black was replaced")
	}
}
