package effects

import (
	"errors"
	"math"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

var (
	// ErrEmptyFrame is returned when a frame contains no colors.
	ErrEmptyFrame = errors.New("empty frame")
	// ErrInvalidFrame is returned when a frame has invalid dimensions or too few colors.
	ErrInvalidFrame = errors.New("invalid frame")
)

// ReductionStrategy defines how many logical colors collapse into one color.
type ReductionStrategy int

const (
	// ReductionFirst uses the first logical color in the reduced range.
	ReductionFirst ReductionStrategy = iota
	// ReductionAverage averages hue circularly and other color fields linearly.
	ReductionAverage
)

// AdaptOptions configures logical frame adaptation to a device surface.
type AdaptOptions struct {
	Reduction ReductionStrategy
}

// DeviceFrame is a packet-independent frame adapted to a device surface.
//
// For matrix devices, ChainIndex identifies the matrix chain, SendWidth/Height
// describe the physical send layout, and Orientation preserves the chain
// orientation for later packet rendering. For non-matrix devices, ChainIndex is
// 0 and Orientation is OrientationRightSideUp.
type DeviceFrame struct {
	ChainIndex  int
	Colors      []Color
	SendWidth   int
	Height      int
	Orientation device.Orientation
	Duration    time.Duration
}

// AdaptFrameToSurface adapts a target-free logical frame to a device surface.
//
// Matrix adaptation samples colors from the logical frame by surface cell
// coordinates. Cells outside the logical frame are padded with blank colors,
// cells outside the surface are cropped, and hidden matrix cells are blanked.
// Adapted matrix DeviceFrames keep packet/send dimensions and do not apply
// orientation; Orientation is carried so a later renderer can rotate for the
// physical device.
func AdaptFrameToSurface(frame Frame, surface device.Surface, opts AdaptOptions) ([]DeviceFrame, error) {
	if err := validateFrame(frame); err != nil {
		return nil, err
	}

	switch surface.LightType {
	case device.LightTypeMultiZone:
		return adaptMultiZoneFrame(frame, surface, opts), nil
	case device.LightTypeMatrix:
		if surface.Matrix != nil && len(surface.Matrix.Chains) > 0 {
			return adaptMatrixFrame(frame, surface), nil
		}
		return adaptRectMatrixFrame(frame, surface), nil
	default:
		return []DeviceFrame{{
			Colors:      []Color{reduceColors(frame.Colors[:frame.Width*frame.Height], opts.Reduction)},
			SendWidth:   1,
			Height:      1,
			Orientation: device.OrientationRightSideUp,
			Duration:    frame.Duration,
		}}, nil
	}
}

func validateFrame(frame Frame) error {
	if len(frame.Colors) == 0 {
		return ErrEmptyFrame
	}
	if frame.Width <= 0 || frame.Height <= 0 || len(frame.Colors) < frame.Width*frame.Height {
		return ErrInvalidFrame
	}
	return nil
}

func adaptMultiZoneFrame(frame Frame, surface device.Surface, opts AdaptOptions) []DeviceFrame {
	zones := max(surface.Zones, surface.Width)
	zones = max(zones, 1)

	colors := make([]Color, zones)
	if frame.Width == zones && frame.Height == 1 {
		copy(colors, frame.Colors[:zones])
	} else if frame.Height == 1 {
		for x := range colors {
			colors[x] = frameColorAt(frame, scaledIndex(x, zones, frame.Width), 0)
		}
	} else {
		for x := range colors {
			sourceX := scaledIndex(x, zones, frame.Width)
			column := make([]Color, frame.Height)
			for y := range frame.Height {
				column[y] = frameColorAt(frame, sourceX, y)
			}
			colors[x] = reduceColors(column, opts.Reduction)
		}
	}

	return []DeviceFrame{{
		Colors:      colors,
		SendWidth:   zones,
		Height:      1,
		Orientation: device.OrientationRightSideUp,
		Duration:    frame.Duration,
	}}
}

func adaptMatrixFrame(frame Frame, surface device.Surface) []DeviceFrame {
	frames := make([]DeviceFrame, 0, len(surface.Matrix.Chains))
	for _, chain := range surface.Matrix.Chains {
		sendWidth := max(chain.SendWidth, 1)
		sendHeight := matrixSendHeight(chain, sendWidth)
		colors := blankColors(sendWidth, sendHeight)
		sendIndex := 0

		for rowIndex, row := range chain.Rows {
			for col := 0; col < row.Cols; col++ {
				if sendIndex >= len(colors) {
					break
				}
				if !hiddenCol(row.HiddenCols, col) {
					x := chain.Bounds.X + row.Offset + col
					y := chain.Bounds.Y + rowIndex
					colors[sendIndex] = frameColorAt(frame, x, y)
				}
				sendIndex++
			}
		}

		frames = append(frames, DeviceFrame{
			ChainIndex:  chain.Index,
			Colors:      colors,
			SendWidth:   sendWidth,
			Height:      sendHeight,
			Orientation: chain.Orientation,
			Duration:    frame.Duration,
		})
	}
	return frames
}

func adaptRectMatrixFrame(frame Frame, surface device.Surface) []DeviceFrame {
	width := max(surface.Width, 1)
	height := max(surface.Height, 1)
	colors := blankColors(width, height)
	for y := range height {
		for x := range width {
			colors[y*width+x] = frameColorAt(frame, x, y)
		}
	}
	return []DeviceFrame{{
		Colors:      colors,
		SendWidth:   width,
		Height:      height,
		Orientation: device.OrientationRightSideUp,
		Duration:    frame.Duration,
	}}
}

func matrixSendHeight(chain device.MatrixChain, sendWidth int) int {
	slots := 0
	for _, row := range chain.Rows {
		slots += max(row.Cols, 0)
	}
	if slots == 0 {
		slots = max(chain.Bounds.Width*chain.Bounds.Height, sendWidth)
	}
	return max((slots+sendWidth-1)/sendWidth, 1)
}

func hiddenCol(hidden []int, col int) bool {
	for _, value := range hidden {
		if value == col {
			return true
		}
	}
	return false
}

func frameColorAt(frame Frame, x, y int) Color {
	if x < 0 || y < 0 || x >= frame.Width || y >= frame.Height {
		return blankColor
	}
	return frame.Colors[y*frame.Width+x]
}

func scaledIndex(index, count, sourceCount int) int {
	if count <= 1 || sourceCount <= 1 {
		return 0
	}
	return min(index*sourceCount/count, sourceCount-1)
}

func reduceColors(colors []Color, strategy ReductionStrategy) Color {
	if len(colors) == 0 {
		return Color{}
	}
	if strategy != ReductionAverage {
		return colors[0]
	}

	var hueX, hueY, saturation, brightness, kelvin float64
	for _, color := range colors {
		radians := color.Hue * math.Pi / 180
		hueX += math.Cos(radians)
		hueY += math.Sin(radians)
		saturation += color.Saturation
		brightness += color.Brightness
		kelvin += float64(color.Kelvin)
	}
	count := float64(len(colors))
	return Color{
		Hue:        averageHue(hueX, hueY),
		Saturation: saturation / count,
		Brightness: brightness / count,
		Kelvin:     uint16(kelvin / count),
	}
}

func averageHue(x, y float64) float64 {
	if math.Abs(x) < 1e-9 && math.Abs(y) < 1e-9 {
		return 0
	}
	hue := math.Atan2(y, x) * 180 / math.Pi
	if hue < 0 {
		hue += 360
	}
	return hue
}
