package device

import (
	"testing"

	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	"github.com/stretchr/testify/assert"
)

func TestNearestOrientation(t *testing.T) {
	tests := map[string]struct {
		pid     uint32
		x, y, z int16
		want    Orientation
	}{
		"invalid": {
			pid: 55,
			x:   -1, y: -1, z: -1,
			want: OrientationRightSideUp,
		},
		"empty field": {
			pid: 55,
			x:   0, y: 0, z: 0,
			want: OrientationRightSideUp,
		},
		"tile: right": {
			pid: 55,
			x:   1800, y: 23, z: 9,
			want: OrientationRight,
		},
		"tile: left": {
			pid: 55,
			x:   -1800, y: 23, z: 9,
			want: OrientationLeft,
		},
		"tile: face down": {
			pid: 55,
			x:   -1, y: 23, z: 900,
			want: OrientationFaceDown,
		},
		"tile: face up": {
			pid: 55,
			x:   -1, y: 23, z: -900,
			want: OrientationFaceUp,
		},
		"tile: upside down": {
			pid: 55,
			x:   -1, y: 230, z: -9,
			want: OrientationUpsideDown,
		},
		"tile: right side up": {
			pid: 55,
			x:   -1, y: -230, z: -9,
			want: OrientationRightSideUp,
		},
		"luna: right": {
			pid: 219,
			x:   1, y: 23, z: -99,
			want: OrientationRight,
		},
		"luna: left": {
			pid: 219,
			x:   1, y: 23, z: 99,
			want: OrientationLeft,
		},
		"luna: face down": {
			pid: 219,
			x:   1, y: 23, z: 9,
			want: OrientationFaceDown,
		},
		"luna: face up": {
			pid: 219,
			x:   1, y: -23, z: 9,
			want: OrientationFaceUp,
		},
		"luna: upside down": {
			pid: 219,
			x:   100, y: -23, z: 9,
			want: OrientationUpsideDown,
		},
		"luna: right side up": {
			pid: 219,
			x:   -100, y: -23, z: 9,
			want: OrientationRightSideUp,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := NearestOrientation(tc.pid, tc.x, tc.y, tc.z)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestReorientSquaredMatrix(t *testing.T) {
	tests := map[string]struct {
		w, h         int
		orientation  Orientation
		colors, want []packets.LightHsbk
	}{
		"upside down": {
			w: 2, h: 2,
			orientation: OrientationUpsideDown,
			colors:      []packets.LightHsbk{{Hue: 0}, {Hue: 60}, {Hue: 120}, {Hue: 180}},
			want:        []packets.LightHsbk{{Hue: 180}, {Hue: 120}, {Hue: 60}, {Hue: 0}},
		},
		"rotated left": {
			w: 2, h: 2,
			orientation: OrientationLeft,
			colors:      []packets.LightHsbk{{Hue: 0}, {Hue: 60}, {Hue: 120}, {Hue: 180}},
			want:        []packets.LightHsbk{{Hue: 120}, {Hue: 0}, {Hue: 180}, {Hue: 60}},
		},
		"rotated right: top-left": {
			w: 2, h: 2,
			orientation: OrientationRight,
			colors:      []packets.LightHsbk{{Hue: 0}, {Hue: 60}, {Hue: 120}, {Hue: 180}},
			want:        []packets.LightHsbk{{Hue: 60}, {Hue: 180}, {Hue: 0}, {Hue: 120}},
		},
		"right side up: no-op": {
			w: 2, h: 2,
			orientation: OrientationRightSideUp,
			colors:      []packets.LightHsbk{{Hue: 0}, {Hue: 60}, {Hue: 120}, {Hue: 180}},
			want:        []packets.LightHsbk{{Hue: 0}, {Hue: 60}, {Hue: 120}, {Hue: 180}},
		},
		"face up: no-op": {
			w: 2, h: 2,
			orientation: OrientationFaceUp,
			colors:      []packets.LightHsbk{{Hue: 0}, {Hue: 60}, {Hue: 120}, {Hue: 180}},
			want:        []packets.LightHsbk{{Hue: 0}, {Hue: 60}, {Hue: 120}, {Hue: 180}},
		},
		"face down: no-op": {
			w: 2, h: 2,
			orientation: OrientationFaceDown,
			colors:      []packets.LightHsbk{{Hue: 0}, {Hue: 60}, {Hue: 120}, {Hue: 180}},
			want:        []packets.LightHsbk{{Hue: 0}, {Hue: 60}, {Hue: 120}, {Hue: 180}},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := ReorientMatrix(tc.w, tc.h, tc.orientation, tc.colors)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRotateMatrix90(t *testing.T) {
	tests := map[string]struct {
		index, w, h int
		want        int
	}{
		"top-left": {
			index: 0, w: 8, h: 8, want: 7,
		},
		"top-right": {
			index: 7, w: 8, h: 8, want: 63,
		},
		"bottom-right": {
			index: 63, w: 8, h: 8, want: 56,
		},
		"bottom-left": {
			index: 56, w: 8, h: 8, want: 0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := RotateMatrix90(tc.w, tc.h)(tc.index)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRotateMatrix180(t *testing.T) {
	tests := map[string]struct {
		index, w, h int
		want        int
	}{
		"top-left": {
			index: 0, w: 8, h: 8, want: 63,
		},
		"top-right": {
			index: 7, w: 8, h: 8, want: 56,
		},
		"bottom-right": {
			index: 63, w: 8, h: 8, want: 0,
		},
		"bottom-left": {
			index: 56, w: 8, h: 8, want: 7,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := RotateMatrix180(tc.w, tc.h)(tc.index)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRotateMatrix270(t *testing.T) {
	tests := map[string]struct {
		index, w, h int
		want        int
	}{
		"top-left": {
			index: 0, w: 8, h: 8, want: 56,
		},
		"top-right": {
			index: 7, w: 8, h: 8, want: 0,
		},
		"bottom-right": {
			index: 63, w: 8, h: 8, want: 7,
		},
		"bottom-left": {
			index: 56, w: 8, h: 8, want: 63,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := RotateMatrix270(tc.w, tc.h)(tc.index)
			assert.Equal(t, tc.want, got)
		})
	}
}

func Test_rotationMapper(t *testing.T) {
	tests := map[string]struct {
		index, w, h int
		orientation Orientation
		want        int
	}{
		"upside down: top-left": {
			index: 0,
			w:     8, h: 8,
			orientation: OrientationUpsideDown,
			want:        63,
		},
		"upside down: top-right": {
			index: 7,
			w:     8, h: 8,
			orientation: OrientationUpsideDown,
			want:        56,
		},
		"rotated left: top-left": {
			index: 0,
			w:     8, h: 8,
			orientation: OrientationLeft,
			want:        7,
		},
		"rotated left: top-right": {
			index: 7,
			w:     8, h: 8,
			orientation: OrientationLeft,
			want:        63,
		},
		"rotated right: top-left": {
			index: 0,
			w:     8, h: 8,
			orientation: OrientationRight,
			want:        56,
		},
		"rotated right: top-right": {
			index: 7,
			w:     8, h: 8,
			orientation: OrientationRight,
			want:        0,
		},
		"right side up: unchanged": {
			index: 0,
			w:     8, h: 8,
			orientation: OrientationRightSideUp,
			want:        0,
		},
		"face up: unchanged": {
			index: 0,
			w:     8, h: 8,
			orientation: OrientationFaceUp,
			want:        0,
		},
		"face down: unchanged": {
			index: 0,
			w:     8, h: 8,
			orientation: OrientationFaceDown,
			want:        0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := rotationMapper(tc.w, tc.h, tc.orientation)(tc.index)
			assert.Equal(t, tc.want, got)
		})
	}
}
