package themes

import (
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/effects"
)

// MatrixLayout is a runtime override for matrix devices only.
type MatrixLayout string

const (
	// MatrixThemeLayout uses the saved Theme layout and axis (also the default).
	MatrixThemeLayout MatrixLayout = "theme"
	// MatrixSpatial blends coherent 2D regions across the logical chain surface.
	// It overrides Theme.Layout and Axis for matrices, but not other light types.
	MatrixSpatial MatrixLayout = "spatial"
)

func variationColors(colors []effects.Color, variation uint64, reverse bool) []effects.Color {
	if variation == 0 && !reverse {
		return colors
	}
	start := int(variation % uint64(len(colors)))
	ordered := make([]effects.Color, len(colors))
	for i := range ordered {
		index := (start + i) % len(colors)
		if reverse {
			index = (start + len(colors) - 1 - i) % len(colors)
		}
		ordered[i] = colors[index]
	}
	return ordered
}

// A small, fixed lattice keeps neighboring cells coherent and costs no
// allocations per pixel. Sample globally across chains, not separately per tile.
func fillSpatialFrame(frame *effects.Frame, colors []effects.Color, serial device.Serial, seed, variation uint64) {
	identity := uint64(0)
	for _, b := range serial {
		identity = identity<<8 | uint64(b)
	}
	random := effects.NewRandom(mixSpatialSeed(seed) ^ mixSpatialSeed(variation^0xd1b54a32d192ed03) ^ mixSpatialSeed(identity^0x94d049bb133111eb))
	var field [3][3]float64
	for y := range field {
		for x := range field[y] {
			field[y][x] = random.Float64()
		}
	}
	for y := 0; y < frame.Height; y++ {
		py := spatialCoordinate(y, frame.Height)
		iy := min(int(py), 1)
		ty := smoothSpatialFraction(py - float64(iy))
		for x := 0; x < frame.Width; x++ {
			px := spatialCoordinate(x, frame.Width)
			ix := min(int(px), 1)
			tx := smoothSpatialFraction(px - float64(ix))
			a := field[iy][ix] + (field[iy][ix+1]-field[iy][ix])*tx
			b := field[iy+1][ix] + (field[iy+1][ix+1]-field[iy+1][ix])*tx
			frame.Colors[y*frame.Width+x] = sampleGradient(colors, a+(b-a)*ty)
		}
	}
}

func spatialCoordinate(position, size int) float64 {
	if size <= 1 {
		return 1 // Sample the middle of a collapsed dimension.
	}
	return 2 * float64(position) / float64(size-1)
}

func smoothSpatialFraction(t float64) float64 { return t * t * (3 - 2*t) }

// Fixed integer mixing keeps seed, variation and identity independent without
// depending on global random state, time, iteration order, or target membership.
func mixSpatialSeed(v uint64) uint64 {
	v += 0x9e3779b97f4a7c15
	v = (v ^ (v >> 30)) * 0xbf58476d1ce4e5b9
	v = (v ^ (v >> 27)) * 0x94d049bb133111eb
	return v ^ (v >> 31)
}
