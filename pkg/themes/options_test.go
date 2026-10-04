package themes

import (
	"math"
	"reflect"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/effects"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func preserveOptions(d device.Device) PlanOptions {
	frame, err := effects.FrameFromDeviceState(d, 0)
	if err != nil {
		panic(err)
	}
	return PlanOptions{Brightness: PreserveBrightness, InitialFrames: map[device.Serial]effects.Frame{d.Serial: frame}}
}

func stripDevice() device.Device {
	d := testDevice(1)
	d.LightType = device.LightTypeMultiZone
	d.Color.Brightness = 99 // Must never be used as the strip's brightness.
	for _, brightness := range []float64{0, 20, 40, 80, 100} {
		d.MultizoneProperties.Zones = append(d.MultizoneProperties.Zones,
			effects.Color{Hue: 120, Saturation: 80, Brightness: brightness, Kelvin: 3500}.ToDeviceColor())
	}
	return d
}

func TestPlanOptionsDefaultMatchesPlan(t *testing.T) {
	theme := testTheme()
	devices := []device.Device{stripDevice(), testDevice(2)}
	want, err := theme.Plan(devices, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for _, opts := range []PlanOptions{{}, {Brightness: PaletteBrightness}} {
		got, err := theme.PlanWithOptions(devices, time.Second, opts)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("got=%v err=%v", got, err)
		}
	}
}

func TestPreserveZoneBrightnessAndOwnership(t *testing.T) {
	d := stripDevice()
	theme := testTheme()
	opts := preserveOptions(d)
	initial := opts.InitialFrames[d.Serial]
	before := slices.Clone(initial.Colors)
	paletteBefore := slices.Clone(theme.Palette.Base)
	physicalBefore := slices.Clone(d.MultizoneProperties.Zones)
	palettePlan, err := theme.Plan([]device.Device{d}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := theme.PlanWithOptions([]device.Device{d}, time.Second, opts)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range plan[0].Frame.Colors {
		want := palettePlan[0].Frame.Colors[i]
		want.Brightness = before[i].Brightness
		if c != want {
			t.Fatalf("cell %d got=%v want=%v", i, c, want)
		}
	}
	if !reflect.DeepEqual(before, initial.Colors) || !reflect.DeepEqual(paletteBefore, theme.Palette.Base) || !reflect.DeepEqual(physicalBefore, d.MultizoneProperties.Zones) {
		t.Fatal("planning mutated inputs")
	}
	plan[0].Frame.Colors[1].Brightness = 99
	if !reflect.DeepEqual(before, initial.Colors) {
		t.Fatal("output aliases initial state")
	}
	initial.Colors[2].Brightness = 99
	if plan[0].Frame.Colors[2].Brightness != before[2].Brightness {
		t.Fatal("input aliases output")
	}
	again, err := theme.PlanWithOptions([]device.Device{d}, time.Second, preserveOptions(d))
	if err != nil || again[0].Frame.Colors[1].Brightness != before[1].Brightness {
		t.Fatal("subsequent plans affected by mutated output")
	}
}

func TestPreserveSingleWhiteOnlyAndTargetAssignment(t *testing.T) {
	a, b := testDevice(1), testDevice(2)
	a.Color = effects.Color{Brightness: 0, Kelvin: 3500}
	b.Color = effects.Color{Brightness: 73, Kelvin: 3500}
	b.ColorProperties = device.ColorProperties{HasColor: false, TemperatureRange: device.TemperatureRange{Min: 5500, Max: 6500}}
	opts := preserveOptions(a)
	opts.InitialFrames[b.Serial] = preserveOptions(b).InitialFrames[b.Serial]
	plan, err := testTheme().PlanWithOptions([]device.Device{b, a}, 0, opts)
	if err != nil {
		t.Fatal(err)
	}
	if plan[0].Serial != a.Serial || plan[0].Frame.Colors[0].Brightness != 0 || plan[0].Frame.Colors[0].Hue != 350 {
		t.Fatal(plan)
	}
	c := plan[1].Frame.Colors[0]
	if c.Brightness != 73 || c.Hue != 10 || c.Saturation != 0 || c.Kelvin != 5500 {
		t.Fatal(c)
	}
	single, err := testTheme().PlanWithOptions([]device.Device{b}, 0, preserveOptions(b))
	if err != nil || single[0].Frame.Colors[0].Hue != 350 {
		t.Fatal("assignment did not reflect changed target set")
	}
}

func TestPreserveMatrixPhysicalMapping(t *testing.T) {
	for _, product := range []uint32{27, 57, 145, 201, 219} {
		t.Run(strconv.Itoa(int(product)), func(t *testing.T) {
			d := testDevice(1)
			d.ProductID = product
			d.LightType = device.LightTypeMatrix
			d.MatrixProperties = device.MatrixProperties{Width: 8, Height: 8, ChainLength: 2, NZones: 64,
				ChainOrientations: []device.Orientation{device.OrientationLeft, device.OrientationRight},
				ChainZones:        make([][]packets.LightHsbk, 2)}
			for chain := range 2 {
				for i := range 64 {
					d.MatrixProperties.ChainZones[chain] = append(d.MatrixProperties.ChainZones[chain],
						effects.Color{Hue: 120, Saturation: 80, Brightness: float64((i + chain*7) % 101), Kelvin: 3500}.ToDeviceColor())
				}
			}
			before := device.CloneMatrixChains(d.MatrixProperties.ChainZones)
			opts := preserveOptions(d)
			initial := opts.InitialFrames[d.Serial]
			plan, err := testTheme().PlanWithOptions([]device.Device{d}, time.Second, opts)
			if err != nil {
				t.Fatal(err)
			}
			for i, c := range plan[0].Frame.Colors {
				if c.Brightness != initial.Colors[i].Brightness {
					t.Fatalf("cell %d", i)
				}
			}
			// Compare through the existing adapters, including hidden emitters,
			// irregular row offsets, orientation, send layout, and chain placement.
			surface := device.SurfaceFromDevice(d)
			want, err := effects.AdaptFrameToSurface(initial, surface, effects.AdaptOptions{})
			if err != nil {
				t.Fatal(err)
			}
			got, err := effects.AdaptFrameToSurface(plan[0].Frame, surface, effects.AdaptOptions{})
			if err != nil {
				t.Fatal(err)
			}
			for chain, frame := range got {
				toPhysical := func(f effects.DeviceFrame) []packets.LightHsbk {
					colors := make([]packets.LightHsbk, len(f.Colors))
					for i, c := range f.Colors {
						colors[i] = c.ToDeviceColor()
					}
					return device.ReorientMatrix(f.SendWidth, f.Height, f.Orientation, colors)
				}
				actual, expected := toPhysical(frame), toPhysical(want[chain])
				if len(actual) != len(expected) {
					t.Fatal("physical dimensions changed")
				}
				for i := range actual {
					if actual[i].Brightness != expected[i].Brightness {
						t.Fatalf("chain %d physical cell %d", chain, i)
					}
					if product == 27 && actual[i].Brightness != before[chain][i].Brightness {
						t.Fatalf("original physical brightness changed: chain %d cell %d", chain, i)
					}
				}
			}
			if !reflect.DeepEqual(before, d.MatrixProperties.ChainZones) {
				t.Fatal("matrix input mutated")
			}
		})
	}
}

func TestInvalidPlanOptions(t *testing.T) {
	d := stripDevice()
	for _, test := range []struct {
		name   string
		change func(*PlanOptions)
	}{
		{"unknown mode", func(o *PlanOptions) { o.Brightness = "average" }},
		{"palette with frames", func(o *PlanOptions) { o.Brightness = PaletteBrightness }},
		{"default with frames", func(o *PlanOptions) { o.Brightness = "" }},
		{"missing frame", func(o *PlanOptions) { o.InitialFrames = nil }},
		{"extra target", func(o *PlanOptions) { o.InitialFrames[testDevice(2).Serial] = o.InitialFrames[d.Serial] }},
		{"wrong dimensions", func(o *PlanOptions) {
			f := o.InitialFrames[d.Serial]
			f.Width = 1
			f.Height = 5
			o.InitialFrames[d.Serial] = f
		}},
		{"short colors", func(o *PlanOptions) {
			f := o.InitialFrames[d.Serial]
			f.Colors = f.Colors[:4]
			o.InitialFrames[d.Serial] = f
		}},
		{"extra colors", func(o *PlanOptions) {
			f := o.InitialFrames[d.Serial]
			f.Colors = append(f.Colors, effects.Color{})
			o.InitialFrames[d.Serial] = f
		}},
		{"nan brightness", func(o *PlanOptions) { o.InitialFrames[d.Serial].Colors[0].Brightness = math.NaN() }},
		{"infinite brightness", func(o *PlanOptions) { o.InitialFrames[d.Serial].Colors[0].Brightness = math.Inf(1) }},
		{"negative brightness", func(o *PlanOptions) { o.InitialFrames[d.Serial].Colors[0].Brightness = -1 }},
		{"excess brightness", func(o *PlanOptions) { o.InitialFrames[d.Serial].Colors[0].Brightness = 101 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			opts := preserveOptions(d)
			test.change(&opts)
			if plan, err := testTheme().PlanWithOptions([]device.Device{d}, 0, opts); err == nil || plan != nil {
				t.Fatalf("plan=%v err=%v", plan, err)
			}
		})
	}
}

func TestExcessiveGeometryFailsBeforeSurfaceConstruction(t *testing.T) {
	for _, geometry := range []device.MatrixProperties{
		{Width: math.MaxInt, Height: math.MaxInt, ChainLength: math.MaxInt},
		{Width: 1, Height: MaxFrameCells + 1, ChainLength: 1},
		{Width: 1, Height: 1, ChainLength: MaxFrameCells + 1},
		{Width: 256, Height: 256, ChainLength: 2},
		{Width: 1, Height: 1, ChainLength: 1, NZones: MaxFrameCells + 1},
		{Width: 1, Height: 1, ChainLength: 1, ChainZones: [][]packets.LightHsbk{make([]packets.LightHsbk, MaxFrameCells+1)}},
	} {
		d := testDevice(1)
		d.LightType = device.LightTypeMatrix
		d.MatrixProperties = geometry
		if plan, err := testTheme().Plan([]device.Device{d}, 0); err == nil || plan != nil {
			t.Fatal("excessive geometry accepted")
		}
	}
	d := stripDevice()
	d.MultizoneProperties.Zones = make([]packets.LightHsbk, MaxFrameCells+1)
	if _, err := testTheme().Plan([]device.Device{d}, 0); err == nil {
		t.Fatal("excessive strip accepted")
	}
	// Metadata-only matrices avoid allocating any planned frames before the
	// aggregate limit is checked. Boundary geometry itself remains supported.
	devices := make([]device.Device, MaxPlanCells/MaxFrameCells+1)
	for i := range devices {
		d := testDevice(byte(i + 1))
		d.LightType = device.LightTypeMatrix
		d.MatrixProperties = device.MatrixProperties{Width: 256, Height: 256, ChainLength: 1}
		devices[i] = d
	}
	if plan, err := testTheme().Plan(devices, 0); err == nil || plan != nil {
		t.Fatal("aggregate limit ignored")
	}
	if _, err := testTheme().Plan(devices[:1], 0); err != nil {
		t.Fatalf("boundary failed: %v", err)
	}
	if _, err := testTheme().Plan(make([]device.Device, MaxPlanTargets+1), 0); err == nil {
		t.Fatal("target count limit ignored")
	}
}

func TestPreserveIgnoresPaletteBrightnessIncludingZero(t *testing.T) {
	d := stripDevice()
	theme := testTheme()
	for i := range theme.Palette.Base {
		theme.Palette.Base[i].Brightness = 0
	}
	opts := preserveOptions(d)
	plan, err := theme.PlanWithOptions([]device.Device{d}, 0, opts)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range plan[0].Frame.Colors {
		if c.Brightness != opts.InitialFrames[d.Serial].Colors[i].Brightness {
			t.Fatal("palette zero overrode initial brightness")
		}
	}
}
