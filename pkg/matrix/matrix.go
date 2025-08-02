package matrix

import (
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

// alignment defines the alignment of a given shape in the matrix.
type alignment int

const (
	AlignLeft alignment = iota
	AlignCenter
	AlignRight
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
		if x >= m.Width {
			if y >= m.MaxY() {
				break
			}
			// Wrap to the beginning of the next line.
			x = 0
			y++
		}
		m.SetPixel(x, y, c)
		x++
	}
}

// SetCorners sets the corners of a matrix to the given palette.
// It accepts up to 4 colors and if fewer are provided it rotates through them.
func (m *Matrix) SetCorners(palette ...packets.LightHsbk) {
	cs := NewColorSlice(4, palette...)
	m.SetPixel(0, 0, cs[0])
	m.SetPixel(m.MaxX(), 0, cs[1])
	m.SetPixel(0, m.MaxY(), cs[2])
	m.SetPixel(m.MaxX(), m.MaxY(), cs[3])
}

// SetHorizontalSegment sets a section of row y to the given palette  starting at column x.
func (m *Matrix) SetHorizontalSegment(x, y, length int, palette ...packets.LightHsbk) {
	length = min(length, m.Width-x)
	m.SetColors(x, y, NewColorSlice(length, palette...)...)
}

// SetVerticalSegment sets a section of a column x to the given palette starting at row y.
func (m *Matrix) SetVerticalSegment(x, y, length int, palette ...packets.LightHsbk) {
	length = min(length, m.Height-y)
	cs := NewColorSlice(length, palette...)
	for i := range length {
		m.SetPixel(x, y+i, cs[i])
	}
}

// SetXForAlignment sets x according to the given alignment.
func (m *Matrix) SetXForAlignment(a alignment, length int) int {
	var x int
	if length < m.Width {
		switch a {
		case AlignCenter:
			x = (m.Width - length) / 2
		case AlignRight:
			x = m.Width - length
		}
	}
	return x
}

// NewColorSlice returns a []packets.LightHsbk of the specific length according to the palette.
// It the palette is shorter than the length then it rotates through it to fill up the slice.
func NewColorSlice(length int, palette ...packets.LightHsbk) []packets.LightHsbk {
	cs := make([]packets.LightHsbk, length)
	if len(palette) > length {
		copy(cs, palette[:length])
		return cs
	}

	// Rotate through the colors to fill the slice.
	for i := range length {
		cs[i] = palette[i%len(palette)]
	}
	return cs
}

// CopyRow applies the colors of a src row to the dst row.
// If dstY or srcY are invalid it does nothing.
func (m *Matrix) CopyRow(dstY, srcY int) {
	if srcY > m.MaxY() || dstY > m.MaxY() {
		return
	}
	copy(m.Colors[dstY], m.Colors[srcY])
}

func (m *Matrix) DrawSquare(x, y, length int, palette ...packets.LightHsbk) {
	m.SetHorizontalSegment(x, y, length, palette...)
	for i := range length - 1 {
		m.CopyRow(y+1+i, y)
	}
}

func (m *Matrix) SetBorder(padding int, colors ...packets.LightHsbk) {
	paddedY := min(padding, m.MaxX()/2, m.MaxY()/2)
	length := m.Width - (padding * 2)
	height := m.Height - (padding * 2)
	x := m.SetXForAlignment(AlignCenter, length)

	// Set top
	m.SetHorizontalSegment(x, paddedY, length, colors...)
	// Set bottom
	m.SetHorizontalSegment(x, m.MaxY()-paddedY, length, colors...)
	// Set left
	m.SetVerticalSegment(x, paddedY, height, colors...)
	// Set right
	m.SetVerticalSegment(m.MaxX()-x, paddedY, height, colors...)
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
