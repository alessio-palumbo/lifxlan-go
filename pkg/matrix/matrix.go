package matrix

import (
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

// Matrix represents a LIFX device with matrix capability.
type Matrix struct {
	Width       int
	Height      int
	Size        int
	Colors      [][]packets.LightHsbk
	ChainLength int
}

// New creates a Matrix of the given size and chain length.
func New(width, height, chainLength int) *Matrix {
	colors := make([][]packets.LightHsbk, height)
	for i := range colors {
		colors[i] = make([]packets.LightHsbk, width)
	}

	return &Matrix{
		Width:       width,
		Height:      height,
		Size:        int(width * height),
		Colors:      colors,
		ChainLength: chainLength,
	}
}

// Clear sets pixels colors to their default value.
// If no pixels are given, it clears the whole matrix.
func (m *Matrix) Clear(pixels ...Pixel) {
	if len(pixels) > 0 {
		for _, p := range pixels {
			m.SetPixel(p.X, p.Y, packets.LightHsbk{})
		}
		return
	}

	for _, r := range m.Colors {
		clear(r)
	}
}

// SetPixel sets a single pixel to the given color
func (m *Matrix) SetPixel(x, y int, c packets.LightHsbk) {
	m.Colors[y][x] = c
}

// SetColors sets the given colors starting at the given position and, if necessary,
// wrapping to the next row.
func (m *Matrix) SetColors(x, y int, cs ...packets.LightHsbk) {
	for _, c := range cs {
		if x >= m.Width && y >= m.MaxY() {
			break
		}
		if x < m.Width {
			m.SetPixel(x, y, c)
			x++
		} else {
			x = 0
			y++
			m.SetPixel(x, y, c)
		}
	}
}

// SetCorners sets the corner
func (m *Matrix) SetCorners(cs [4]packets.LightHsbk) {
	m.SetPixel(0, 0, cs[0])
	m.SetPixel(m.MaxX(), 0, cs[1])
	m.SetPixel(0, m.MaxY(), cs[2])
	m.SetPixel(m.MaxX(), m.MaxY(), cs[3])
}

// MaxX is the last valid row index.
func (m *Matrix) MaxX() int {
	return m.Width - 1
}

// MaxY is the last valid column index.
func (m *Matrix) MaxY() int {
	return m.Height - 1
}

// FlattenColors converts the Colors' matrix into a 64-byte array that can be
// used with the LIFX protocol.
func (m *Matrix) FlattenColors() [64]packets.LightHsbk {
	var colors [64]packets.LightHsbk
	for y := range m.Height {
		for x := range m.Width {
			colors[y*m.Height+x] = m.Colors[y][x]
		}
	}
	return colors
}
