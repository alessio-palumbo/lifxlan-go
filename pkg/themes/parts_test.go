package themes

import (
	"context"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/effects"
	"github.com/alessio-palumbo/lifxlan-go/pkg/effects/adapters"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func ceilingState(pid uint32, w, h int, orientation device.Orientation) device.Device {
	d := testDevice(1)
	d.ProductID, d.LightType = pid, device.LightTypeMatrix
	d.MatrixProperties = device.MatrixProperties{Width: w, Height: h, ChainLength: 1, NZones: w * h, ChainOrientations: []device.Orientation{orientation}, ChainZones: [][]packets.LightHsbk{make([]packets.LightHsbk, w*h)}}
	for i := range w * h {
		d.MatrixProperties.ChainZones[0][i] = packets.LightHsbk{Hue: uint16(1234 + i), Saturation: 34567, Brightness: uint16(i * 401), Kelvin: 3500}
	}
	return d
}

func partPacketColors(t *testing.T, a PartApplication, cells int) []packets.LightHsbk {
	t.Helper()
	var out []packets.LightHsbk
	for _, m := range a.Messages {
		if p, ok := m.Payload.(*packets.TileSet64); ok {
			out = append(out, p.Colors[:]...)
		}
		if m.Type() == 21 || m.Type() == 117 {
			t.Fatal("theme changed power")
		}
	}
	if len(out) < cells {
		t.Fatal("missing physical colors")
	}
	return out[:cells]
}

func TestPartThemesSelectionsGeometryAndOwnership(t *testing.T) {
	for _, geometry := range []struct {
		pid        uint32
		w, h, main int
	}{{265, 8, 8, 56}, {201, 8, 16, 120}, {201, 16, 8, 120}} {
		for _, orientation := range []device.Orientation{device.OrientationRightSideUp, device.OrientationLeft, device.OrientationRight} {
			for _, part := range []device.LightPart{device.LightPartAll, device.LightPartMain, device.LightPartUplight} {
				d := ceilingState(geometry.pid, geometry.w, geometry.h, orientation)
				before := d.Clone()
				theme := testTheme()
				plan, err := theme.PlanParts([]device.Device{d}, time.Second, PartPlanOptions{Part: part})
				if err != nil {
					t.Fatalf("pid=%d orientation=%v part=%s: %v", geometry.pid, orientation, part, err)
				}
				got := partPacketColors(t, plan[0], geometry.w*geometry.h)
				mainMask, _ := effects.LightPartPhysicalMask(d, device.LightPartMain)
				upMask, _ := effects.LightPartPhysicalMask(d, device.LightPartUplight)
				mainCells, upCells := 0, 0
				for i, c := range got {
					selected := part != device.LightPartUplight && mainMask[i].Brightness != 0 || part != device.LightPartMain && upMask[i].Brightness != 0
					if !selected && c != before.MatrixProperties.ChainZones[0][i] {
						t.Fatalf("unselected physical cell %d changed", i)
					}
					if mainMask[i].Brightness != 0 {
						mainCells++
					}
					if upMask[i].Brightness != 0 {
						upCells++
						if part != device.LightPartMain && c != plan[0].Uplight.ToDeviceColor() {
							t.Fatal("uplight preview does not match packet")
						}
					}
				}
				if mainCells != geometry.main || upCells != 1 || !reflect.DeepEqual(d, before) {
					t.Fatal("mapping or input mutation")
				}
				if part == device.LightPartMain && plan[0].Uplight != nil {
					t.Fatal("unexpected uplight update")
				}
				got[0].Hue++
				plan[0].Frame.Colors[0].Hue++
				if !reflect.DeepEqual(d, before) {
					t.Fatal("aliased input")
				}
			}
		}
	}
}

func TestPartThemesDeterministicVariationAndMainMapping(t *testing.T) {
	a, b := ceilingState(265, 8, 8, device.OrientationLeft), ceilingState(265, 8, 8, device.OrientationRight)
	b.Serial[5]++
	theme := testTheme()
	opts := PartPlanOptions{Variation: 1, Reverse: true, Seed: 42, MatrixLayout: MatrixSpatial}
	first, err := theme.PlanParts([]device.Device{b, a}, time.Second, opts)
	if err != nil {
		t.Fatal(err)
	}
	again, err := theme.PlanParts([]device.Device{a, b}, time.Second, opts)
	if err != nil || !reflect.DeepEqual(first, again) {
		t.Fatal("non-deterministic part plan", err)
	}
	colors := variationColors(theme.colors(), opts.Variation, opts.Reverse)
	for i, p := range first {
		if p.Part != device.LightPartAll || *p.Uplight != colors[i%len(colors)] {
			t.Fatal("incorrect palette distribution")
		}
		d := a
		if p.Serial == b.Serial {
			d = b
		}
		physical := partPacketColors(t, p, 64)
		var legacy []*protocol.Message
		r := adapters.NewRendererForDevice(d, func(m *protocol.Message) error { legacy = append(legacy, m); return nil })
		if err := r.RenderFrame(context.Background(), p.Frame); err != nil {
			t.Fatal(err)
		}
		want := partPacketColors(t, PartApplication{Messages: legacy}, 64)
		mask, _ := effects.LightPartPhysicalMask(d, device.LightPartMain)
		for j := range physical {
			if mask[j].Brightness != 0 && physical[j] != want[j] {
				t.Fatalf("main layout changed at %d", j)
			}
		}
	}
	opts.Variation = 2
	changed, err := theme.PlanParts([]device.Device{a, b}, time.Second, opts)
	if err != nil || first[0].Uplight.Hue == changed[0].Uplight.Hue {
		t.Fatal("variation did not advance uplight")
	}
}

func TestPartThemesPreserveExactPhysicalBrightnessIncludingBlack(t *testing.T) {
	for _, part := range []device.LightPart{device.LightPartAll, device.LightPartMain, device.LightPartUplight} {
		d := ceilingState(265, 8, 8, device.OrientationLeft)
		upMask, _ := effects.LightPartPhysicalMask(d, device.LightPartUplight)
		for i, c := range upMask {
			if c.Brightness != 0 {
				d.MatrixProperties.ChainZones[0][i].Brightness = 0
			}
		}
		plan, err := testTheme().PlanParts([]device.Device{d}, 0, PartPlanOptions{Part: part, Brightness: PreserveBrightness})
		if err != nil {
			t.Fatal(err)
		}
		physical := partPacketColors(t, plan[0], 64)
		for i, c := range physical {
			if c.Brightness != d.MatrixProperties.ChainZones[0][i].Brightness {
				t.Fatalf("physical brightness %d changed", i)
			}
		}
		if plan[0].Uplight != nil && plan[0].Uplight.Brightness != 0 {
			t.Fatal("black uplight not preserved")
		}
	}
	d := testDevice(1)
	d.Color.Brightness = 73
	plan, err := testTheme().PlanParts([]device.Device{d}, 0, PartPlanOptions{Brightness: PreserveBrightness})
	if err != nil || plan[0].Frame.Colors[0].Brightness != 73 {
		t.Fatal("single-zone brightness", err)
	}
}

func TestPartThemesOrdinaryTargetsMatchLegacy(t *testing.T) {
	for _, d := range []device.Device{testDevice(1), stripDevice(), ceilingState(27, 8, 8, device.OrientationRight)} {
		for _, part := range []device.LightPart{device.LightPartAll, device.LightPartMain} {
			plan, err := testTheme().PlanParts([]device.Device{d}, time.Second, PartPlanOptions{Part: part})
			if err != nil {
				t.Fatal(err)
			}
			legacy, err := testTheme().Plan([]device.Device{d}, time.Second)
			if err != nil || !reflect.DeepEqual(plan[0].Application, legacy[0]) || plan[0].Uplight != nil {
				t.Fatal("legacy frame changed", err)
			}
			var want []*protocol.Message
			r := adapters.NewRendererForDevice(d, func(m *protocol.Message) error { want = append(want, m); return nil })
			if err := r.RenderFrame(context.Background(), legacy[0].Frame); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(plan[0].Messages, want) {
				t.Fatal("legacy packets changed")
			}
		}
	}
}

func TestPartThemesPreserveOrdinaryZonesAndMatrixChains(t *testing.T) {
	d := stripDevice()
	for i := range d.MultizoneProperties.Zones {
		d.MultizoneProperties.Zones[i].Brightness = uint16(i * 713)
	}
	plan, err := testTheme().PlanParts([]device.Device{d}, 0, PartPlanOptions{Brightness: PreserveBrightness})
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range plan[0].Frame.Colors {
		if c.ToDeviceColor().Brightness != d.MultizoneProperties.Zones[i].Brightness {
			t.Fatalf("zone %d brightness quantized", i)
		}
	}
	d = ceilingState(27, 8, 8, device.OrientationLeft)
	d.MatrixProperties.ChainLength = 2
	d.MatrixProperties.ChainZones = append(d.MatrixProperties.ChainZones, device.CloneHSBKs(d.MatrixProperties.ChainZones[0]))
	d.MatrixProperties.ChainOrientations = []device.Orientation{device.OrientationLeft, device.OrientationRight}
	for i := range d.MatrixProperties.ChainZones[1] {
		d.MatrixProperties.ChainZones[1][i].Brightness = uint16(i * 613)
	}
	plan, err = testTheme().PlanParts([]device.Device{d}, 0, PartPlanOptions{Brightness: PreserveBrightness})
	if err != nil {
		t.Fatal(err)
	}
	for _, msg := range plan[0].Messages {
		if p, ok := msg.Payload.(*packets.TileSet64); ok {
			for i, c := range p.Colors {
				if c.Brightness != d.MatrixProperties.ChainZones[int(p.TileIndex)][i].Brightness {
					t.Fatalf("chain %d cell %d brightness quantized", p.TileIndex, i)
				}
			}
		}
	}
}

func TestPartThemesInvalidStateAndOptions(t *testing.T) {
	d := ceilingState(265, 8, 8, device.OrientationRightSideUp)
	for _, opts := range []PartPlanOptions{{Part: "side"}, {Brightness: "average"}, {MatrixLayout: "random"}} {
		if _, err := testTheme().PlanParts([]device.Device{d}, 0, opts); err == nil {
			t.Fatal("invalid options accepted")
		}
	}
	if _, err := testTheme().PlanParts([]device.Device{testDevice(1)}, 0, PartPlanOptions{Part: device.LightPartUplight}); err == nil {
		t.Fatal("unsupported uplight accepted")
	}
	for _, mutate := range []func(*device.Device){
		func(d *device.Device) { d.MatrixProperties.ChainZones = nil },
		func(d *device.Device) { d.MatrixProperties.ChainZones[0] = d.MatrixProperties.ChainZones[0][:63] },
		func(d *device.Device) { d.MatrixProperties.ChainLength = 2 },
		func(d *device.Device) { d.MatrixProperties.Width = math.MaxInt },
	} {
		bad := d.Clone()
		mutate(&bad)
		if _, err := testTheme().PlanParts([]device.Device{bad}, 0, PartPlanOptions{}); err == nil {
			t.Fatal("invalid initial geometry accepted")
		}
	}
	white := d.Clone()
	white.ColorProperties.HasColor = false
	white.ColorProperties.TemperatureRange = device.TemperatureRange{Min: 5500, Max: 6500}
	plan, err := testTheme().PlanParts([]device.Device{white}, 0, PartPlanOptions{})
	if err != nil || plan[0].Uplight.Saturation != 0 || plan[0].Uplight.Kelvin != 5500 {
		t.Fatal("white adaptation", err)
	}
}
