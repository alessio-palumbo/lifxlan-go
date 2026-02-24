package device

import (
	"math"

	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

// Orientation specifies the orientation of a matrix according to accelerometer.
type Orientation uint8

// Valid orientation values.
const (
	OrientationRightSideUp Orientation = iota
	OrientationUpsideDown
	OrientationFaceUp
	OrientationFaceDown
	OrientationLeft
	OrientationRight
)

// Name of groups sharing the same orientationAxis.
const (
	orientationGroupTile = "Tile"
	orientationGroupLuna = "Luna"
)

// orientationAxis defines the Orientation value according to the greatest accelerometer value.
type orientationAxis struct {
	XPos Orientation
	XNeg Orientation
	YPos Orientation
	YNeg Orientation
	ZPos Orientation
	ZNeg Orientation
}

// orientationByGroup matches groups name to orientationAxis configuration.
var orientationByGroup = map[string]orientationAxis{
	orientationGroupTile: {
		XPos: OrientationRight,
		XNeg: OrientationLeft,
		YPos: OrientationUpsideDown,
		YNeg: OrientationRightSideUp,
		ZPos: OrientationFaceDown,
		ZNeg: OrientationFaceUp,
	},
	orientationGroupLuna: {
		XPos: OrientationUpsideDown,
		XNeg: OrientationRightSideUp,
		YPos: OrientationFaceDown,
		YNeg: OrientationFaceUp,
		ZPos: OrientationLeft,
		ZNeg: OrientationRight,
	},
}

// orientationGroupByPID matches productID with groups that share orientationAxis.
var orientationGroupByPID = map[uint32]string{
	55:  orientationGroupTile,
	219: orientationGroupLuna,
	220: orientationGroupLuna,
}

// NearestOrientation determines the nearest orientation direction value from the given
// tile accelerometer vector components.
func NearestOrientation(pid uint32, x, y, z int16) Orientation {
	absX := math.Abs(float64(x))
	absY := math.Abs(float64(y))
	absZ := math.Abs(float64(z))

	if x == y && y == z {
		// Invalid data, assume right-side up.
		return OrientationRightSideUp
	}

	switch {
	case absX > absY && absX > absZ:
		if x > 0 {
			return orientationByGroup[orientationGroupByPID[pid]].XPos
		}
		return orientationByGroup[orientationGroupByPID[pid]].XNeg
	case absZ > absX && absZ > absY:
		if z > 0 {
			return orientationByGroup[orientationGroupByPID[pid]].ZPos
		}
		return orientationByGroup[orientationGroupByPID[pid]].ZNeg
	default:
		if y > 0 {
			return orientationByGroup[orientationGroupByPID[pid]].YPos
		}
		return orientationByGroup[orientationGroupByPID[pid]].YNeg
	}
}

// ReorientMatrix rotates the colors of a matrix w x h accounting for orientation.
func ReorientMatrix(w, h int, o Orientation, colors []packets.LightHsbk) []packets.LightHsbk {
	if o == OrientationRightSideUp || o == OrientationFaceUp || o == OrientationFaceDown {
		return colors
	}
	return RotateMatrix(rotationMapper(w, h, o), colors)
}

// RotateMatrix returns a new []packets.LightHsbk processed by the rotate function given.
func RotateMatrix(rotate func(i int) int, colors []packets.LightHsbk) []packets.LightHsbk {
	out := make([]packets.LightHsbk, len(colors))
	for i := range colors {
		out[rotate(i)] = colors[i]
	}
	return out
}

// RotateMatrix90 returns a func that performs a 90 degrees rotation of a given index for a w x h matrix.
func RotateMatrix90(w, h int) func(i int) int {
	maxH := h - 1
	return func(i int) int {
		x, y := i%w, i/w
		x, y = maxH-y, x
		return x + y*h
	}
}

// RotateMatrix180 returns a func that performs a 180 degrees rotation of a given index for a w x h matrix.
func RotateMatrix180(w, h int) func(i int) int {
	maxW, maxH := w-1, h-1
	return func(i int) int {
		x, y := i%w, i/w
		x, y = maxW-x, maxH-y
		return x + y*w
	}
}

// RotateMatrix270 returns a func that performs a 270 degrees rotation of a given index for a w x h matrix.
func RotateMatrix270(w, h int) func(i int) int {
	maxW := w - 1
	return func(i int) int {
		x, y := i%w, i/w
		x, y = y, maxW-x
		return x + y*h
	}
}

// rotationMapper returns a func that performs a rotation on a given index according to matrix orientation.
func rotationMapper(w, h int, o Orientation) func(int) int {
	switch o {
	case OrientationUpsideDown:
		return RotateMatrix180(w, h)
	case OrientationLeft:
		return RotateMatrix90(w, h)
	case OrientationRight:
		return RotateMatrix270(w, h)
	}
	return func(i int) int { return i }
}
