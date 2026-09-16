package effects

import (
	"fmt"
	"math"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

// PhysicalColorState stores exact HSBK values in device packet order.
//
// Zones is used by single-zone and multizone surfaces. MatrixChains is used by
// matrix surfaces and is indexed by MatrixChain.Index. Matrix colors include
// hidden/non-emitting physical slots; those slots are omitted when adapting
// the state to a logical Frame.
//
// The state does not model power, transitions, multizone apply buffering, or
// matrix frame buffers. An emulator can keep one state value per frame buffer.
type PhysicalColorState struct {
	Zones        []packets.LightHsbk
	MatrixChains [][]packets.LightHsbk
}

// NewPhysicalColorState returns blank physical storage sized for surface.
// Returned slices are newly allocated and do not alias surface or other state.
func NewPhysicalColorState(surface device.Surface) PhysicalColorState {
	blank := blankColor.ToDeviceColor()

	if surface.LightType != device.LightTypeMatrix {
		zones := 1
		if surface.LightType == device.LightTypeMultiZone {
			zones = max(surface.Zones, surface.Width, 1)
		}
		return PhysicalColorState{Zones: filledDeviceColors(zones, blank)}
	}

	if surface.Matrix == nil || len(surface.Matrix.Chains) == 0 {
		return PhysicalColorState{MatrixChains: [][]packets.LightHsbk{
			filledDeviceColors(max(surface.Width, 1)*max(surface.Height, 1), blank),
		}}
	}

	maxChainIndex := -1
	for _, chain := range surface.Matrix.Chains {
		maxChainIndex = max(maxChainIndex, chain.Index)
	}
	if maxChainIndex < 0 {
		return PhysicalColorState{}
	}

	chains := make([][]packets.LightHsbk, maxChainIndex+1)
	for _, chain := range surface.Matrix.Chains {
		if chain.Index < 0 {
			continue
		}
		width := max(chain.SendWidth, 1)
		height := matrixSendHeight(chain, width)
		chains[chain.Index] = filledDeviceColors(width*height, blank)
	}
	return PhysicalColorState{MatrixChains: chains}
}

// MergeZoneColors copies a single-zone or multizone physical update into state
// starting at start. Colors beyond the allocated physical range are cropped.
// The input slice is copied and is not retained.
func (s *PhysicalColorState) MergeZoneColors(start int, colors []packets.LightHsbk) error {
	if s == nil || len(s.Zones) == 0 {
		return fmt.Errorf("%w: missing physical zones", ErrInvalidPhysicalState)
	}
	if start < 0 {
		return fmt.Errorf("%w: negative zone start %d", ErrInvalidPhysicalUpdate, start)
	}
	if start >= len(s.Zones) || len(colors) == 0 {
		return nil
	}
	copy(s.Zones[start:], colors)
	return nil
}

// MergeMatrixColors copies a rectangular physical update into one matrix
// chain. x and y are physical coordinates and width is the row width carried
// by the update, such as TileSet64.Rect.Width. The destination dimensions and
// chain orientation come from surface. Cells outside the physical send bounds
// are cropped. The input slice is copied and is not retained.
//
// A packet addressing multiple chains should be expanded by the caller into
// one call per addressed chain.
func (s *PhysicalColorState) MergeMatrixColors(surface device.Surface, chainIndex, x, y, width int, colors []packets.LightHsbk) error {
	if s == nil {
		return fmt.Errorf("%w: nil physical state", ErrInvalidPhysicalState)
	}
	if x < 0 || y < 0 || width <= 0 {
		return fmt.Errorf("%w: matrix region x=%d y=%d width=%d", ErrInvalidPhysicalUpdate, x, y, width)
	}

	chain, ok := matrixChainByIndex(surface, chainIndex)
	if !ok || chainIndex < 0 || chainIndex >= len(s.MatrixChains) || len(s.MatrixChains[chainIndex]) == 0 {
		return fmt.Errorf("%w: missing matrix chain %d", ErrInvalidPhysicalState, chainIndex)
	}

	sendWidth := max(chain.SendWidth, 1)
	sendHeight := matrixSendHeight(chain, sendWidth)
	destination := s.MatrixChains[chainIndex]
	for sourceIndex, color := range colors {
		destinationX := x + sourceIndex%width
		destinationY := y + sourceIndex/width
		if destinationX < 0 || destinationX >= sendWidth || destinationY < 0 || destinationY >= sendHeight {
			continue
		}
		destinationIndex := destinationY*sendWidth + destinationX
		if destinationIndex < len(destination) {
			destination[destinationIndex] = color
		}
	}
	return nil
}

// AdaptPhysicalColorStateToFrame adapts exact physical HSBK state into a
// normalized logical/display Frame for surface.
//
// Matrix orientation is removed here. Chain bounds, row offsets, and hidden
// cells are interpreted in logical coordinates. Physical values outside the
// surface are cropped; missing values and logical cells without a physical
// emitter are padded with BlankColor. Mapped HSBK values retain enough
// precision for Color.ToDeviceColor to recover the original protocol values.
// The returned Frame owns its color slice and does not alias state.
func AdaptPhysicalColorStateToFrame(state PhysicalColorState, surface device.Surface, duration time.Duration) (Frame, error) {
	return adaptPhysicalColorStateToFrame(state, surface, duration, blankColor.ToDeviceColor(), exactDeviceColor)
}

func adaptPhysicalColorStateToFrame(state PhysicalColorState, surface device.Surface, duration time.Duration, physicalPadding packets.LightHsbk, decode func(packets.LightHsbk) Color) (Frame, error) {
	switch surface.LightType {
	case device.LightTypeMultiZone:
		zones := max(surface.Zones, surface.Width, 1)
		return logicalZoneFrame(state.Zones, zones, duration, decode), nil
	case device.LightTypeMatrix:
		return logicalMatrixFrame(state, surface, duration, physicalPadding, decode)
	default:
		return logicalZoneFrame(state.Zones, 1, duration, decode), nil
	}
}

func logicalZoneFrame(physical []packets.LightHsbk, zones int, duration time.Duration, decode func(packets.LightHsbk) Color) Frame {
	frame := NewFrame(zones, 1, duration, blankColor)
	for i, color := range physical {
		if i >= len(frame.Colors) {
			break
		}
		frame.Colors[i] = decode(color)
	}
	return frame
}

func logicalMatrixFrame(state PhysicalColorState, surface device.Surface, duration time.Duration, physicalPadding packets.LightHsbk, decode func(packets.LightHsbk) Color) (Frame, error) {
	width := max(surface.Width, 1)
	height := max(surface.Height, 1)
	frame := NewFrame(width, height, duration, blankColor)

	if surface.Matrix == nil || len(surface.Matrix.Chains) == 0 {
		if len(state.MatrixChains) > 0 {
			for i, color := range state.MatrixChains[0] {
				if i >= len(frame.Colors) {
					break
				}
				frame.Colors[i] = decode(color)
			}
		}
		return frame, nil
	}

	for _, chain := range surface.Matrix.Chains {
		if chain.Index < 0 {
			return Frame{}, fmt.Errorf("%w: negative matrix chain index %d", ErrInvalidPhysicalState, chain.Index)
		}
		sendWidth := max(chain.SendWidth, 1)
		sendHeight := matrixSendHeight(chain, sendWidth)
		physical := physicalChainColors(state, chain.Index, sendWidth*sendHeight, physicalPadding)
		logical := device.PhysicalMatrixColorsToLogical(sendWidth, sendHeight, chain.Orientation, physical)

		forEachMatrixCell(chain, len(logical), func(physicalIndex, x, y int, hidden bool) {
			if !hidden {
				SetFrameColor(&frame, x, y, decode(logical[physicalIndex]))
			}
		})
	}
	return frame, nil
}

func physicalChainColors(state PhysicalColorState, chainIndex, size int, padding packets.LightHsbk) []packets.LightHsbk {
	colors := filledDeviceColors(size, padding)
	if chainIndex >= 0 && chainIndex < len(state.MatrixChains) {
		copy(colors, state.MatrixChains[chainIndex])
	}
	return colors
}

func matrixChainByIndex(surface device.Surface, index int) (device.MatrixChain, bool) {
	if surface.Matrix == nil || len(surface.Matrix.Chains) == 0 {
		if index == 0 {
			return device.MatrixChain{
				Index:     0,
				Bounds:    device.Rect{Width: max(surface.Width, 1), Height: max(surface.Height, 1)},
				SendWidth: max(surface.Width, 1),
			}, true
		}
		return device.MatrixChain{}, false
	}
	for _, chain := range surface.Matrix.Chains {
		if chain.Index == index {
			return chain, true
		}
	}
	return device.MatrixChain{}, false
}

func filledDeviceColors(count int, color packets.LightHsbk) []packets.LightHsbk {
	colors := make([]packets.LightHsbk, max(count, 0))
	for i := range colors {
		colors[i] = color
	}
	return colors
}

// exactDeviceColor keeps sufficient precision for Color.ToDeviceColor to
// recover every HSBK protocol field exactly.
func exactDeviceColor(color packets.LightHsbk) Color {
	return Color{
		Hue:        float64(color.Hue) / math.MaxUint16 * 360,
		Saturation: float64(color.Saturation) / math.MaxUint16 * 100,
		Brightness: float64(color.Brightness) / math.MaxUint16 * 100,
		Kelvin:     color.Kelvin,
	}
}
