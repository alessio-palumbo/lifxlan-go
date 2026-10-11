package effects

import (
	"fmt"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	productregistry "github.com/alessio-palumbo/lifxregistry-go/gen/registry"
	"math"
)

// MaxPartFrameCells bounds physical and logical part-frame allocations.
const MaxPartFrameCells = 65_536

// ComposeLightPartFrame merges optional main and uplight colors into an independent
// physical frame. Nil leaves a part untouched; preserveBrightness retains exact
// physical brightness on selected cells. Hidden/padding cells are always retained.
// No network or power operations occur. The caller must establish complete receipt
// coverage before supplying d: allocated buffers cannot prove observation.
func ComposeLightPartFrame(d device.Device, main *Frame, uplight *Color, preserveBrightness bool) ([]packets.LightHsbk, error) {
	mainMask, err := LightPartPhysicalMask(d, device.LightPartMain)
	if err != nil {
		return nil, err
	}
	upMask, err := LightPartPhysicalMask(d, device.LightPartUplight)
	if err != nil {
		return nil, err
	}
	valid := func(c Color) bool {
		return !math.IsNaN(c.Hue) && !math.IsInf(c.Hue, 0) && c.Hue >= 0 && c.Hue <= 360 &&
			!math.IsNaN(c.Saturation) && !math.IsInf(c.Saturation, 0) && c.Saturation >= 0 && c.Saturation <= 100 &&
			!math.IsNaN(c.Brightness) && !math.IsInf(c.Brightness, 0) && c.Brightness >= 0 && c.Brightness <= 100 && c.Kelvin >= 1500 && c.Kelvin <= 9000
	}
	var mapped []packets.LightHsbk
	if main != nil {
		surface := device.SurfaceFromDevice(d)
		if main.Width != surface.Width || main.Height != surface.Height || len(main.Colors) != main.Width*main.Height {
			return nil, fmt.Errorf("main frame must match %dx%d surface exactly", surface.Width, surface.Height)
		}
		for _, c := range main.Colors {
			if !valid(c) {
				return nil, fmt.Errorf("invalid main frame color")
			}
		}
		frames, err := AdaptFrameToSurface(*main, surface, AdaptOptions{})
		if err != nil {
			return nil, err
		}
		f := frames[0]
		colors := make([]packets.LightHsbk, len(f.Colors))
		for i, c := range f.Colors {
			colors[i] = c.ToDeviceColor()
		}
		mapped = device.LogicalMatrixColorsToPhysical(f.SendWidth, f.Height, f.Orientation, colors)
		if len(mapped) != len(mainMask) {
			return nil, fmt.Errorf("main frame/state geometry mismatch")
		}
	}
	if uplight != nil && !valid(*uplight) {
		return nil, fmt.Errorf("invalid uplight color")
	}
	out := device.CloneHSBKs(d.MatrixProperties.ChainZones[0])
	for i, old := range out {
		if main != nil && mainMask[i].Brightness != 0 {
			out[i] = mapped[i]
		}
		if uplight != nil && upMask[i].Brightness != 0 {
			out[i] = uplight.ToDeviceColor()
		}
		if preserveBrightness {
			out[i].Brightness = old.Brightness
		}
	}
	return out, nil
}

// LightPartPhysicalMask maps main or uplight onto physical matrix cells.
// A nonzero brightness marks a selected emitter. It requires complete physical
// geometry, not proof of receipt; callers must supply observed state.
// Current registry uplights support single-chain matrices only.
func LightPartPhysicalMask(d device.Device, part device.LightPart) ([]packets.LightHsbk, error) {
	if part != device.LightPartMain && part != device.LightPartUplight {
		return nil, fmt.Errorf("mask requires main or uplight selection")
	}
	p, known := productregistry.ProductsByPID[int(d.ProductID)]
	if !known || p.Features.UplightCoords == nil || d.LightType != device.LightTypeMatrix {
		return nil, fmt.Errorf("uplight mapping unavailable")
	}
	m := d.MatrixProperties
	// Current registry uplights belong to single-chain ceilings. Avoid guessing
	// how future chained/multi-part products will address their uplights.
	if m.ChainLength != 1 || len(m.ChainZones) != 1 || m.Width <= 0 || m.Height <= 0 || m.Width > 255 || m.Height > 255 || m.Width > MaxPartFrameCells/m.Height || len(m.ChainZones[0]) != m.Width*m.Height {
		return nil, fmt.Errorf("incomplete or unsupported matrix geometry")
	}
	if len(m.ChainZones[0]) > 64 && 64%m.Width != 0 {
		return nil, fmt.Errorf("matrix chunks must align with rows")
	}
	surface := device.SurfaceFromDevice(d)
	if surface.Width <= 0 || surface.Height <= 0 || surface.Width > MaxPartFrameCells/surface.Height {
		return nil, fmt.Errorf("excessive logical surface")
	}
	chain := &surface.Matrix.Chains[0]
	coords := p.Features.UplightCoords
	if coords.Y < 0 || coords.Y >= len(chain.Rows) || coords.X < 0 || coords.X >= chain.Rows[coords.Y].Cols {
		return nil, fmt.Errorf("uplight coordinate outside surface")
	}
	maskColor := BlankColor()
	maskColor.Brightness = 0
	mask := NewFrame(surface.Width, surface.Height, 0, maskColor)
	for rowIndex, row := range chain.Rows {
		for col := 0; col < row.Cols; col++ {
			up := rowIndex == coords.Y && col == coords.X
			if up {
				continue
			}
			if part == device.LightPartMain {
				color := maskColor
				color.Brightness = 100
				SetFrameColor(&mask, chain.Bounds.X+row.Offset+col, chain.Bounds.Y+rowIndex, color)
			}
		}
	}
	if part == device.LightPartUplight {
		// Reveal only the auxiliary emitter in our private mask surface. The
		// normal main surface/hidden-cell mapping remains unchanged.
		row := &chain.Rows[coords.Y]
		var hidden []int
		for _, col := range row.HiddenCols {
			if col != coords.X {
				hidden = append(hidden, col)
			}
		}
		row.HiddenCols = hidden
		color := maskColor
		color.Brightness = 100
		SetFrameColor(&mask, chain.Bounds.X+row.Offset+coords.X, chain.Bounds.Y+coords.Y, color)
	}
	frames, err := AdaptFrameToSurface(mask, surface, AdaptOptions{})
	if err != nil {
		return nil, err
	}
	frame := frames[0]
	logical := make([]packets.LightHsbk, len(frame.Colors))
	for i, color := range frame.Colors {
		logical[i] = color.ToDeviceColor()
	}
	physicalMask := device.LogicalMatrixColorsToPhysical(frame.SendWidth, frame.Height, frame.Orientation, logical)
	if len(physicalMask) != len(m.ChainZones[0]) {
		return nil, fmt.Errorf("mask/state geometry mismatch")
	}
	return physicalMask, nil
}
