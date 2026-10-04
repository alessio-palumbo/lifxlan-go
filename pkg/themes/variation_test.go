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

func variedTheme() Theme {
	return Theme{Name: "Regions", Palette: effects.Palette{Base: []effects.Color{
		{Hue: 0, Saturation: 80, Brightness: 20, Kelvin: 3500},
		{Hue: 40, Saturation: 80, Brightness: 40, Kelvin: 4000},
		{Hue: 80, Saturation: 80, Brightness: 60, Kelvin: 4500},
	}}}
}

func variationMatrix(product uint32) device.Device {
	d := testDevice(10)
	d.ProductID = product
	d.LightType = device.LightTypeMatrix
	d.MatrixProperties = device.MatrixProperties{Width: 8, Height: 8, ChainLength: 2, NZones: 64,
		ChainOrientations: []device.Orientation{device.OrientationLeft, device.OrientationRight},
		ChainZones:        [][]packets.LightHsbk{make([]packets.LightHsbk, 64), make([]packets.LightHsbk, 64)}}
	for chain := range d.MatrixProperties.ChainZones {
		for i := range d.MatrixProperties.ChainZones[chain] {
			d.MatrixProperties.ChainZones[chain][i] = effects.Color{Brightness: float64((i + chain*7) % 100), Kelvin: 3500}.ToDeviceColor()
		}
	}
	return d
}

func TestSingleZoneVariationsCycleAndDistribute(t *testing.T) {
	theme := variedTheme()
	devices := []device.Device{testDevice(3), stripDevice(), testDevice(2), testDevice(4)}
	for variation := uint64(0); variation < 7; variation++ {
		for _, reverse := range []bool{false, true} {
			opts := PlanOptions{Variation: variation, Reverse: reverse}
			plan, err := theme.PlanWithOptions(devices, 0, opts)
			if err != nil {
				t.Fatal(err)
			}
			index := 0
			for _, a := range plan {
				if a.Serial == devices[1].Serial {
					continue
				}
				want := int(variation%3) + index
				if reverse {
					want = int(variation%3) + 2 - index
				}
				want %= 3
				if a.Frame.Colors[0] != theme.Palette.Base[want] {
					t.Fatalf("variation=%d reverse=%v target=%v", variation, reverse, a.Serial)
				}
				index++
			}
			shuffled := slices.Clone(devices)
			slices.Reverse(shuffled)
			again, err := theme.PlanWithOptions(shuffled, 0, opts)
			if err != nil || !reflect.DeepEqual(plan, again) {
				t.Fatal("target order changed variation")
			}
		}
	}
	// Very large caller-owned variation counters wrap without integer overflow.
	plan, err := theme.PlanWithOptions([]device.Device{testDevice(1)}, 0, PlanOptions{Variation: math.MaxUint64})
	if err != nil || plan[0].Frame.Colors[0] != theme.Palette.Base[math.MaxUint64%3] {
		t.Fatal("large variation failed")
	}
}

func TestMultizoneOffsetReverseAndSmoothGradient(t *testing.T) {
	d := stripDevice()
	d.MultizoneProperties.Zones = make([]packets.LightHsbk, 17)
	theme := variedTheme()
	for _, opts := range []PlanOptions{{Variation: 1}, {Reverse: true}, {Variation: 2, Reverse: true}} {
		plan, err := theme.PlanWithOptions([]device.Device{d}, time.Second, opts)
		if err != nil {
			t.Fatal(err)
		}
		ordered := variationColors(theme.colors(), opts.Variation, opts.Reverse)
		colors := plan[0].Frame.Colors
		if colors[0] != ordered[0] || colors[8] != ordered[1] || colors[16] != ordered[2] {
			t.Fatal("offset or reverse incorrect")
		}
		for i := 1; i < len(colors); i++ {
			if math.Abs(colors[i].Hue-colors[i-1].Hue) > 10.000001 {
				t.Fatal("gradient is not smooth")
			}
		}
		seeded, err := theme.PlanWithOptions([]device.Device{d}, time.Second, PlanOptions{Variation: opts.Variation, Reverse: opts.Reverse, Seed: 123})
		if err != nil || !reflect.DeepEqual(plan, seeded) {
			t.Fatal("spatial seed affected strip")
		}
	}
	// Reversal must swap endpoints even when there are only two palette stops.
	theme.Palette.Base = theme.Palette.Base[:2]
	forward, err := theme.Plan([]device.Device{d}, 0)
	if err != nil {
		t.Fatal(err)
	}
	backward, err := theme.PlanWithOptions([]device.Device{d}, 0, PlanOptions{Reverse: true})
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range backward[0].Frame.Colors {
		if c != forward[0].Frame.Colors[len(forward[0].Frame.Colors)-1-i] {
			t.Fatal("two-stop reversal failed")
		}
	}
}

func TestVariationsPreserveBrightnessAcrossLightTypes(t *testing.T) {
	single := testDevice(2)
	single.Color.Brightness = 73
	for _, d := range []device.Device{single, stripDevice(), variationMatrix(27)} {
		opts := preserveOptions(d)
		opts.Variation, opts.Reverse, opts.Seed, opts.MatrixLayout = math.MaxUint64, true, 17, MatrixSpatial
		plan, err := variedTheme().PlanWithOptions([]device.Device{d}, 0, opts)
		if err != nil {
			t.Fatal(err)
		}
		for i, c := range plan[0].Frame.Colors {
			if c.Brightness != opts.InitialFrames[d.Serial].Colors[i].Brightness {
				t.Fatal("variation changed brightness")
			}
		}
	}
}

func TestSpatialMatrixDeterminismCoherenceAndIdentity(t *testing.T) {
	d := variationMatrix(27)
	theme := variedTheme()
	opts := PlanOptions{Variation: 1, Seed: 42, MatrixLayout: MatrixSpatial}
	plan, err := theme.PlanWithOptions([]device.Device{d}, 0, opts)
	if err != nil {
		t.Fatal(err)
	}
	again, err := theme.PlanWithOptions([]device.Device{d}, 0, opts)
	if err != nil || !reflect.DeepEqual(plan, again) {
		t.Fatal("not deterministic")
	}
	for _, changed := range []PlanOptions{{Variation: 2, Seed: 42, MatrixLayout: MatrixSpatial}, {Variation: 1, Seed: 43, MatrixLayout: MatrixSpatial}} {
		other, err := theme.PlanWithOptions([]device.Device{d}, 0, changed)
		if err != nil || reflect.DeepEqual(plan, other) {
			t.Fatal("seed/variation did not vary matrix")
		}
	}
	f := plan[0].Frame
	var horizontal, vertical bool
	for y := 0; y < f.Height; y++ {
		for x := 0; x < f.Width; x++ {
			c := f.Colors[y*f.Width+x]
			if x > 0 {
				delta := math.Abs(c.Brightness - f.Colors[y*f.Width+x-1].Brightness)
				horizontal = horizontal || delta > .001
				if delta > 80*3/float64(f.Width-1)+1e-9 {
					t.Fatal("neighboring columns lack coherence")
				}
			}
			if y > 0 {
				delta := math.Abs(c.Brightness - f.Colors[(y-1)*f.Width+x].Brightness)
				vertical = vertical || delta > .001
				if delta > 80*3/float64(f.Height-1)+1e-9 {
					t.Fatal("neighboring rows lack coherence")
				}
			}
		}
	}
	if !horizontal || !vertical {
		t.Fatal("not a two-dimensional pattern")
	}
	group, err := theme.PlanWithOptions([]device.Device{testDevice(1), d}, 0, opts)
	if err != nil || !reflect.DeepEqual(plan[0], group[1]) {
		t.Fatal("unrelated group target changed matrix")
	}
	// Runtime matrix override must not modify the definition or honor its axis.
	theme.Layout, theme.Axis = Solid, Vertical
	overridden, err := theme.PlanWithOptions([]device.Device{d}, 0, opts)
	if err != nil || !reflect.DeepEqual(plan, overridden) {
		t.Fatal("spatial override depended on saved layout")
	}
}

func TestSpatialIrregularMatricesAndPreservedBrightness(t *testing.T) {
	for _, product := range []uint32{27, 57, 145, 201, 219} {
		t.Run(strconv.Itoa(int(product)), func(t *testing.T) {
			d := variationMatrix(product)
			before := device.CloneMatrixChains(d.MatrixProperties.ChainZones)
			opts := preserveOptions(d)
			opts.Variation, opts.Seed, opts.MatrixLayout = 5, 71, MatrixSpatial
			theme := variedTheme()
			paletteBefore := slices.Clone(theme.Palette.Base)
			plan, err := theme.PlanWithOptions([]device.Device{d}, 0, opts)
			if err != nil {
				t.Fatal(err)
			}
			initial := opts.InitialFrames[d.Serial]
			for i, c := range plan[0].Frame.Colors {
				if c.Brightness != initial.Colors[i].Brightness {
					t.Fatalf("brightness changed at cell %d", i)
				}
			}
			surface := device.SurfaceFromDevice(d)
			actual, err := effects.AdaptFrameToSurface(plan[0].Frame, surface, effects.AdaptOptions{})
			if err != nil {
				t.Fatal(err)
			}
			expected, err := effects.AdaptFrameToSurface(initial, surface, effects.AdaptOptions{})
			if err != nil {
				t.Fatal(err)
			}
			for chain, f := range actual {
				if f.Orientation != expected[chain].Orientation || f.SendWidth != expected[chain].SendWidth || f.ChainIndex != expected[chain].ChainIndex {
					t.Fatal("mapping changed")
				}
				for i, c := range f.Colors {
					if c.Brightness != expected[chain].Colors[i].Brightness {
						t.Fatal("physical brightness changed")
					}
				}
			}
			plan[0].Frame.Colors[0].Brightness = 99
			if !reflect.DeepEqual(before, d.MatrixProperties.ChainZones) || !reflect.DeepEqual(paletteBefore, theme.Palette.Base) {
				t.Fatal("inputs mutated")
			}
			fresh, err := theme.PlanWithOptions([]device.Device{d}, 0, opts)
			if err != nil || fresh[0].Frame.Colors[0].Brightness != initial.Colors[0].Brightness {
				t.Fatal("output aliases input")
			}
		})
	}
}

func TestVariationDefaultsAndValidation(t *testing.T) {
	devices := []device.Device{stripDevice(), testDevice(2), variationMatrix(57)}
	for _, layout := range []Layout{"", Gradient, Steps, Solid} {
		for _, axis := range []Axis{"", Horizontal, Vertical} {
			theme := variedTheme()
			theme.Layout, theme.Axis = layout, axis
			want, err := theme.Plan(devices, 0)
			if err != nil {
				t.Fatal(err)
			}
			got, err := theme.PlanWithOptions(devices, 0, PlanOptions{MatrixLayout: MatrixThemeLayout, Seed: 100})
			if err != nil || !reflect.DeepEqual(want, got) {
				t.Fatal("default layout behavior changed")
			}
		}
	}
	if plan, err := variedTheme().PlanWithOptions(devices, 0, PlanOptions{MatrixLayout: "noise"}); err == nil || plan != nil {
		t.Fatal("unknown matrix layout accepted")
	}
	// A single stop and degenerate surfaces must remain valid and uniform.
	theme := variedTheme()
	theme.Palette.Base = theme.Palette.Base[:1]
	d := variationMatrix(27)
	d.MatrixProperties = device.MatrixProperties{Width: 1, Height: 1, ChainLength: 1}
	plan, err := theme.PlanWithOptions([]device.Device{d}, 0, PlanOptions{Variation: math.MaxUint64, MatrixLayout: MatrixSpatial})
	if err != nil || plan[0].Frame.Colors[0] != theme.Palette.Base[0] {
		t.Fatal("degenerate spatial frame failed")
	}
}
