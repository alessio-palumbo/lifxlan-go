package matrix

import (
	"testing"

	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	"github.com/stretchr/testify/assert"
)

func TestClear(t *testing.T) {
	clearMatrix := New(4, 4, 0).Colors
	testCases := map[string]struct {
		setColors func(*Matrix)
		pixels    []Pixel
		want      [][]packets.LightHsbk
	}{
		"clear all": {
			setColors: func(m *Matrix) {
				m.SetColors(0, 0, packets.LightHsbk{Kelvin: 3500}, packets.LightHsbk{Kelvin: 3700}, packets.LightHsbk{Kelvin: 3600})
			},
			want: clearMatrix,
		},
		"clear by pixels": {
			setColors: func(m *Matrix) {
				m.SetColors(0, 0, packets.LightHsbk{Kelvin: 3500}, packets.LightHsbk{Kelvin: 3700}, packets.LightHsbk{Kelvin: 3600})
			},
			pixels: []Pixel{{0, 0}, {1, 0}, {2, 0}},
			want:   clearMatrix,
		},
		"clear some pixels": {
			setColors: func(m *Matrix) {
				m.SetColors(0, 0, packets.LightHsbk{Kelvin: 3500}, packets.LightHsbk{Kelvin: 3700}, packets.LightHsbk{Kelvin: 3600})
			},
			pixels: []Pixel{{0, 0}},
			want: [][]packets.LightHsbk{
				{{}, packets.LightHsbk{Kelvin: 3700}, packets.LightHsbk{Kelvin: 3600}, {}},
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			m := New(4, 4, 0)
			tc.setColors(m)
			m.Clear(tc.pixels...)
			assert.Equal(t, m.Colors, tc.want)
		})
	}
}

func TestSetColor(t *testing.T) {
	testCases := map[string]struct {
		setColors func(*Matrix)
		want      [][]packets.LightHsbk
	}{
		"colors shorter than width": {
			setColors: func(m *Matrix) {
				m.SetColors(0, 0, packets.LightHsbk{Kelvin: 3500}, packets.LightHsbk{Kelvin: 3700}, packets.LightHsbk{Kelvin: 3600})
			},
			want: [][]packets.LightHsbk{
				{packets.LightHsbk{Kelvin: 3500}, packets.LightHsbk{Kelvin: 3700}, packets.LightHsbk{Kelvin: 3600}, {}},
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
			},
		},
		"wraps around when colors is greater than width": {
			setColors: func(m *Matrix) {
				m.SetColors(
					0, 0,
					packets.LightHsbk{Kelvin: 3500}, packets.LightHsbk{Kelvin: 3700}, packets.LightHsbk{Kelvin: 3600},
					packets.LightHsbk{Kelvin: 3500}, packets.LightHsbk{Kelvin: 3700}, packets.LightHsbk{Kelvin: 3600},
				)
			},
			want: [][]packets.LightHsbk{
				{packets.LightHsbk{Kelvin: 3500}, packets.LightHsbk{Kelvin: 3700}, packets.LightHsbk{Kelvin: 3600}, packets.LightHsbk{Kelvin: 3500}},
				{packets.LightHsbk{Kelvin: 3700}, packets.LightHsbk{Kelvin: 3600}, {}, {}},
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
			},
		},
		"does not exceed matrix size": {
			setColors: func(m *Matrix) {
				m.SetColors(2, 3, packets.LightHsbk{Kelvin: 3500}, packets.LightHsbk{Kelvin: 3700}, packets.LightHsbk{Kelvin: 3600})
			},
			want: [][]packets.LightHsbk{
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
				{{}, {}, packets.LightHsbk{Kelvin: 3500}, packets.LightHsbk{Kelvin: 3700}},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			m := New(4, 4, 0)
			tc.setColors(m)
			assert.Equal(t, m.Colors, tc.want)
		})
	}
}

func TestSetCorners(t *testing.T) {
	testCases := map[string]struct {
		palette []packets.LightHsbk
		want    [][]packets.LightHsbk
	}{
		"set corners with same color": {
			palette: []packets.LightHsbk{{Kelvin: 3500}},
			want: [][]packets.LightHsbk{
				{packets.LightHsbk{Kelvin: 3500}, {}, {}, packets.LightHsbk{Kelvin: 3500}},
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
				{packets.LightHsbk{Kelvin: 3500}, {}, {}, packets.LightHsbk{Kelvin: 3500}},
			},
		},
		"set corners with fewer than 4 colors": {
			palette: []packets.LightHsbk{{Kelvin: 3500}, {Kelvin: 3600}},
			want: [][]packets.LightHsbk{
				{packets.LightHsbk{Kelvin: 3500}, {}, {}, packets.LightHsbk{Kelvin: 3600}},
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
				{packets.LightHsbk{Kelvin: 3500}, {}, {}, packets.LightHsbk{Kelvin: 3600}},
			},
		},
		"set corners with greater than 4 colors": {
			palette: []packets.LightHsbk{{Kelvin: 3500}, {Kelvin: 3600}, {Kelvin: 3700}, {Kelvin: 3800}, {Kelvin: 3900}},
			want: [][]packets.LightHsbk{
				{packets.LightHsbk{Kelvin: 3500}, {}, {}, packets.LightHsbk{Kelvin: 3600}},
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
				{packets.LightHsbk{Kelvin: 3700}, {}, {}, packets.LightHsbk{Kelvin: 3800}},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			m := New(4, 4, 0)
			m.SetCorners(tc.palette...)
			assert.Equal(t, m.Colors, tc.want)
		})
	}
}

func TestSetHorizontalSegment(t *testing.T) {
	testCases := map[string]struct {
		alignment alignment
		row       int
		length    int
		palette   []packets.LightHsbk
		want      [][]packets.LightHsbk
	}{
		"align left": {
			alignment: AlignLeft,
			length:    2,
			palette:   []packets.LightHsbk{{Kelvin: 3500}, {Kelvin: 3500}},
			want: [][]packets.LightHsbk{
				{{Kelvin: 3500}, {Kelvin: 3500}, {}, {}},
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
			},
		},
		"align right": {
			alignment: AlignRight,
			row:       1,
			length:    2,
			palette:   []packets.LightHsbk{{Kelvin: 3500}, {Kelvin: 3500}},
			want: [][]packets.LightHsbk{
				{{}, {}, {}, {}},
				{{}, {}, {Kelvin: 3500}, {Kelvin: 3500}},
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
			},
		},
		"align center": {
			alignment: AlignCenter,
			row:       3,
			length:    2,
			palette:   []packets.LightHsbk{{Kelvin: 3500}, {Kelvin: 3500}},
			want: [][]packets.LightHsbk{
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
				{{}, {Kelvin: 3500}, {Kelvin: 3500}, {}},
			},
		},
		"palette greater than length": {
			alignment: AlignLeft,
			length:    3,
			palette:   []packets.LightHsbk{{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}},
			want: [][]packets.LightHsbk{
				{{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {}},
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
			},
		},
		"palette shorter than length": {
			alignment: AlignLeft,
			length:    3,
			palette:   []packets.LightHsbk{{Kelvin: 3500}, {Kelvin: 3700}},
			want: [][]packets.LightHsbk{
				{{Kelvin: 3500}, {Kelvin: 3700}, {Kelvin: 3500}, {}},
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			m := New(4, 4, 0)
			x := m.SetXForAlignment(tc.alignment, tc.length)
			m.SetHorizontalSegment(x, tc.row, tc.length, tc.palette...)
			assert.Equal(t, m.Colors, tc.want)
		})
	}
}

func TestSetVerticalSegment(t *testing.T) {
	testCases := map[string]struct {
		col     int
		row     int
		length  int
		palette []packets.LightHsbk
		want    [][]packets.LightHsbk
	}{
		"offset col": {
			col:     2,
			length:  2,
			palette: []packets.LightHsbk{{Kelvin: 3500}, {Kelvin: 3500}},
			want: [][]packets.LightHsbk{
				{{}, {}, {Kelvin: 3500}, {}},
				{{}, {}, {Kelvin: 3500}, {}},
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
			},
		},
		"offset row": {
			row:     2,
			length:  2,
			palette: []packets.LightHsbk{{Kelvin: 3500}, {Kelvin: 3500}},
			want: [][]packets.LightHsbk{
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
				{{Kelvin: 3500}, {}, {}, {}},
				{{Kelvin: 3500}, {}, {}, {}},
			},
		},
		"palette greater than length": {
			length:  3,
			palette: []packets.LightHsbk{{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}},
			want: [][]packets.LightHsbk{
				{{Kelvin: 3500}, {}, {}, {}},
				{{Kelvin: 3500}, {}, {}, {}},
				{{Kelvin: 3500}, {}, {}, {}},
				{{}, {}, {}, {}},
			},
		},
		"palette shorter than length": {
			length:  3,
			palette: []packets.LightHsbk{{Kelvin: 3500}, {Kelvin: 3700}},
			want: [][]packets.LightHsbk{
				{{Kelvin: 3500}, {}, {}, {}},
				{{Kelvin: 3700}, {}, {}, {}},
				{{Kelvin: 3500}, {}, {}, {}},
				{{}, {}, {}, {}},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			m := New(4, 4, 0)
			m.SetVerticalSegment(tc.col, tc.row, tc.length, tc.palette...)
			assert.Equal(t, m.Colors, tc.want)
		})
	}
}

func TestSetXForAlignment(t *testing.T) {
	testCases := map[string]struct {
		matrix    *Matrix
		alignment alignment
		length    int
		want      int
	}{
		"align left": {
			matrix:    New(4, 4, 0),
			alignment: AlignLeft,
			length:    2,
			want:      0,
		},
		"align center": {
			matrix:    New(4, 4, 0),
			alignment: AlignCenter,
			length:    2,
			want:      1,
		},
		"align right": {
			matrix:    New(4, 4, 0),
			alignment: AlignRight,
			length:    2,
			want:      2,
		},
		"align center: odd widht": {
			matrix:    New(7, 5, 0),
			alignment: AlignCenter,
			length:    5,
			want:      1,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := tc.matrix.SetXForAlignment(tc.alignment, tc.length)
			assert.Equal(t, got, tc.want)
		})
	}
}

func TestNewColorsSlice(t *testing.T) {
	testCases := map[string]struct {
		length  int
		palette []packets.LightHsbk
		want    []packets.LightHsbk
	}{
		"palette greater than length": {
			length: 4,
			palette: []packets.LightHsbk{
				{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500},
				{Kelvin: 3500}, {Kelvin: 3500},
			},
			want: []packets.LightHsbk{
				{Kelvin: 3500}, {Kelvin: 3500},
				{Kelvin: 3500}, {Kelvin: 3500},
			},
		},
		"palette equal length": {
			length: 4,
			palette: []packets.LightHsbk{
				{Kelvin: 3500}, {Kelvin: 3500},
				{Kelvin: 3500}, {Kelvin: 3500},
			},
			want: []packets.LightHsbk{
				{Kelvin: 3500}, {Kelvin: 3500},
				{Kelvin: 3500}, {Kelvin: 3500},
			},
		},
		"palette shorter than length": {
			length: 4,
			palette: []packets.LightHsbk{
				{Kelvin: 3500}, {Kelvin: 3700},
			},
			want: []packets.LightHsbk{
				{Kelvin: 3500}, {Kelvin: 3700},
				{Kelvin: 3500}, {Kelvin: 3700},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := NewColorSlice(tc.length, tc.palette...)
			assert.Equal(t, got, tc.want)
		})
	}
}

func TestCopyRow(t *testing.T) {
	testCases := map[string]struct {
		dstY, srcY int
		want       [][]packets.LightHsbk
	}{
		"copies previous row": {
			srcY: 0,
			dstY: 1,
			want: [][]packets.LightHsbk{
				{{Kelvin: 3500}, {Kelvin: 3500}, {}, {}},
				{{Kelvin: 3500}, {Kelvin: 3500}, {}, {}},
				{{Kelvin: 3500}, {Kelvin: 3500}, {}, {}},
				{{}, {}, {}, {}},
			},
		},
		"copies from bottom row": {
			srcY: 3,
			dstY: 0,
			want: [][]packets.LightHsbk{
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
				{{Kelvin: 3500}, {Kelvin: 3500}, {}, {}},
				{{}, {}, {}, {}},
			},
		},
		"skips copy with invalid srcY": {
			srcY: 5,
			dstY: 1,
			want: [][]packets.LightHsbk{
				{{Kelvin: 3500}, {Kelvin: 3500}, {}, {}},
				{{}, {}, {}, {}},
				{{Kelvin: 3500}, {Kelvin: 3500}, {}, {}},
				{{}, {}, {}, {}},
			},
		},
		"skips copy with invalid dstY": {
			srcY: 0,
			dstY: 5,
			want: [][]packets.LightHsbk{
				{{Kelvin: 3500}, {Kelvin: 3500}, {}, {}},
				{{}, {}, {}, {}},
				{{Kelvin: 3500}, {Kelvin: 3500}, {}, {}},
				{{}, {}, {}, {}},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			m := New(4, 4, 0)
			m.Colors = [][]packets.LightHsbk{
				{{Kelvin: 3500}, {Kelvin: 3500}, {}, {}},
				{{}, {}, {}, {}},
				{{Kelvin: 3500}, {Kelvin: 3500}, {}, {}},
				{{}, {}, {}, {}},
			}
			m.CopyRow(tc.dstY, tc.srcY)
			assert.Equal(t, m.Colors, tc.want)
		})
	}
}
func TestDrawSquare(t *testing.T) {
	testCases := map[string]struct {
		alignment alignment
		row       int
		length    int
		palette   []packets.LightHsbk
		want      [][]packets.LightHsbk
	}{
		"align left": {
			alignment: AlignLeft,
			length:    2,
			palette:   []packets.LightHsbk{{Kelvin: 3500}, {Kelvin: 3500}},
			want: [][]packets.LightHsbk{
				{{Kelvin: 3500}, {Kelvin: 3500}, {}, {}},
				{{Kelvin: 3500}, {Kelvin: 3500}, {}, {}},
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
			},
		},
		"align right": {
			alignment: AlignRight,
			row:       1,
			length:    2,
			palette:   []packets.LightHsbk{{Kelvin: 3500}, {Kelvin: 3500}},
			want: [][]packets.LightHsbk{
				{{}, {}, {}, {}},
				{{}, {}, {Kelvin: 3500}, {Kelvin: 3500}},
				{{}, {}, {Kelvin: 3500}, {Kelvin: 3500}},
				{{}, {}, {}, {}},
			},
		},
		"align center": {
			alignment: AlignCenter,
			row:       2,
			length:    2,
			palette:   []packets.LightHsbk{{Kelvin: 3500}, {Kelvin: 3500}},
			want: [][]packets.LightHsbk{
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
				{{}, {Kelvin: 3500}, {Kelvin: 3500}, {}},
				{{}, {Kelvin: 3500}, {Kelvin: 3500}, {}},
			},
		},
		"palette greater than length": {
			alignment: AlignLeft,
			length:    3,
			palette:   []packets.LightHsbk{{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}},
			want: [][]packets.LightHsbk{
				{{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {}},
				{{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {}},
				{{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {}},
				{{}, {}, {}, {}},
			},
		},
		"palette shorter than length": {
			alignment: AlignLeft,
			length:    3,
			palette:   []packets.LightHsbk{{Kelvin: 3500}, {Kelvin: 3700}},
			want: [][]packets.LightHsbk{
				{{Kelvin: 3500}, {Kelvin: 3700}, {Kelvin: 3500}, {}},
				{{Kelvin: 3500}, {Kelvin: 3700}, {Kelvin: 3500}, {}},
				{{Kelvin: 3500}, {Kelvin: 3700}, {Kelvin: 3500}, {}},
				{{}, {}, {}, {}},
			},
		},
		"row overflows": {
			alignment: AlignLeft,
			row:       2,
			length:    3,
			palette:   []packets.LightHsbk{{Kelvin: 3500}},
			want: [][]packets.LightHsbk{
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
				{{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {}},
				{{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {}},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			m := New(4, 4, 0)
			x := m.SetXForAlignment(tc.alignment, tc.length)
			m.DrawSquare(x, tc.row, tc.length, tc.palette...)
			assert.Equal(t, m.Colors, tc.want)
		})
	}
}

func TestSetBorder(t *testing.T) {
	testCases := map[string]struct {
		matrix  *Matrix
		padding int
		palette []packets.LightHsbk
		want    [][]packets.LightHsbk
	}{
		"outer border": {
			matrix:  New(4, 4, 0),
			palette: []packets.LightHsbk{{Kelvin: 3500}},
			want: [][]packets.LightHsbk{
				{{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}},
				{{Kelvin: 3500}, {}, {}, {Kelvin: 3500}},
				{{Kelvin: 3500}, {}, {}, {Kelvin: 3500}},
				{{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}},
			},
		},
		"with padding": {
			matrix:  New(4, 4, 0),
			padding: 1,
			palette: []packets.LightHsbk{{Kelvin: 3500}},
			want: [][]packets.LightHsbk{
				{{}, {}, {}, {}},
				{{}, {Kelvin: 3500}, {Kelvin: 3500}, {}},
				{{}, {Kelvin: 3500}, {Kelvin: 3500}, {}},
				{{}, {}, {}, {}},
			},
		},
		"pads irregular matrix": {
			matrix:  New(7, 5, 0),
			padding: 1,
			palette: []packets.LightHsbk{{Kelvin: 3500}},
			want: [][]packets.LightHsbk{
				{{}, {}, {}, {}, {}, {}, {}},
				{{}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {}},
				{{}, {Kelvin: 3500}, {}, {}, {}, {Kelvin: 3500}, {}},
				{{}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {}},
				{{}, {}, {}, {}, {}, {}, {}},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.matrix.SetBorder(tc.padding, tc.palette...)
			assert.Equal(t, tc.matrix.Colors, tc.want)
		})
	}
}

func TestFlattenColors(t *testing.T) {
	color0 := packets.LightHsbk{Kelvin: 3500}
	testCases := map[string]struct {
		matrix *Matrix
		setter func(m *Matrix)
		want   [64]packets.LightHsbk
	}{
		"start": {
			matrix: New(4, 4, 0),
			setter: func(m *Matrix) { m.SetColors(0, 0, color0) },
			want:   [64]packets.LightHsbk{color0},
		},
		"middle": {
			matrix: New(4, 4, 0),
			setter: func(m *Matrix) { m.SetColors(2, 1, color0) },
			want: [64]packets.LightHsbk{
				{}, {}, {}, {},
				{}, {}, color0,
			},
		},
		"end": {
			matrix: New(4, 4, 0),
			setter: func(m *Matrix) { m.SetColors(3, 3, color0) },
			want: [64]packets.LightHsbk{
				{}, {}, {}, {},
				{}, {}, {}, {},
				{}, {}, {}, {},
				{}, {}, {}, color0,
			},
		},
		"smallest": {
			matrix: New(2, 2, 0),
			setter: func(m *Matrix) { m.SetColors(1, 0, color0) },
			want: [64]packets.LightHsbk{
				{}, color0,
				{}, {},
			},
		},
		"irregular: larger width": {
			matrix: New(4, 2, 0),
			setter: func(m *Matrix) { m.SetColors(3, 0, color0) },
			want: [64]packets.LightHsbk{
				{}, {}, {}, color0,
			},
		},
		"irregular: larger height": {
			matrix: New(2, 4, 0),
			setter: func(m *Matrix) { m.SetColors(1, 1, color0) },
			want: [64]packets.LightHsbk{
				{}, {},
				{}, color0,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.setter(tc.matrix)
			assert.Equal(t, tc.matrix.FlattenColors(), tc.want)
		})
	}
}

func TestParseColors(t *testing.T) {
	testCases := map[string]struct {
		matrix *Matrix
		colors [64]packets.LightHsbk
		want   [][]packets.LightHsbk
	}{
		"parses colors": {
			matrix: New(4, 4, 0),
			colors: [64]packets.LightHsbk{
				{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500},
				{}, {Kelvin: 3500}, {Kelvin: 3500}, {},
				{Kelvin: 3500}, {Kelvin: 3500}, {}, {},
				{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500},
			},
			want: [][]packets.LightHsbk{
				{{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}},
				{{}, {Kelvin: 3500}, {Kelvin: 3500}, {}},
				{{Kelvin: 3500}, {Kelvin: 3500}, {}, {}},
				{{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}},
			},
		},
		"parses colors: offset": {
			matrix: New(4, 4, 0),
			colors: [64]packets.LightHsbk{
				{}, {}, {}, {},
				{}, {}, {}, {},
				{}, {}, {}, {},
				{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500},
				{}, {Kelvin: 3500}, {Kelvin: 3500}, {},
			},
			want: [][]packets.LightHsbk{
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
				{{}, {}, {}, {}},
				{{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.matrix.ParseColors(tc.colors)
			assert.Equal(t, tc.matrix.Colors, tc.want)
		})
	}
}
