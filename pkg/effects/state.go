package effects

import (
	"fmt"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

// FrameFromDeviceState converts cached device color state into a logical Frame.
//
// The function is pure: it does not query the network or mutate d. Matrix
// devices are converted through device.SurfaceFromDevice so chain layout,
// hidden cells, send width, and orientation are interpreted consistently with
// AdaptFrameToSurface.
func FrameFromDeviceState(d device.Device, duration time.Duration) (Frame, error) {
	switch d.LightType {
	case device.LightTypeMultiZone:
		return frameFromMultizoneState(d, duration)
	case device.LightTypeMatrix:
		return frameFromMatrixState(d, duration)
	default:
		return NewFrame(1, 1, duration, d.Color), nil
	}
}

func frameFromMultizoneState(d device.Device, duration time.Duration) (Frame, error) {
	if len(d.MultizoneProperties.Zones) == 0 {
		return Frame{}, fmt.Errorf("%w: missing multizone colors", ErrInvalidDeviceState)
	}
	colors := make([]Color, len(d.MultizoneProperties.Zones))
	for i, color := range d.MultizoneProperties.Zones {
		colors[i] = device.NewColor(color)
	}
	return Frame{Colors: colors, Width: len(colors), Height: 1, Duration: duration}, nil
}

func frameFromMatrixState(d device.Device, duration time.Duration) (Frame, error) {
	surface := device.SurfaceFromDevice(d)
	if surface.Width <= 0 || surface.Height <= 0 {
		return Frame{}, fmt.Errorf("%w: invalid matrix surface", ErrInvalidDeviceState)
	}
	if surface.Matrix == nil || len(surface.Matrix.Chains) == 0 {
		return rectangularMatrixFrameFromState(d, surface, duration)
	}
	if len(d.MatrixProperties.ChainZones) == 0 {
		return Frame{}, fmt.Errorf("%w: missing matrix colors", ErrInvalidDeviceState)
	}

	frame := NewFrame(surface.Width, surface.Height, duration, blankColor)
	for _, chain := range surface.Matrix.Chains {
		if chain.Index < 0 || chain.Index >= len(d.MatrixProperties.ChainZones) || len(d.MatrixProperties.ChainZones[chain.Index]) == 0 {
			return Frame{}, fmt.Errorf("%w: missing matrix chain %d colors", ErrInvalidDeviceState, chain.Index)
		}

		sendWidth := max(chain.SendWidth, 1)
		sendHeight := matrixSendHeight(chain, sendWidth)
		colors := matrixStateColors(d.MatrixProperties.ChainZones[chain.Index], sendWidth, sendHeight, chain.Orientation)
		sendIndex := 0

		for rowIndex, row := range chain.Rows {
			for col := 0; col < row.Cols; col++ {
				if sendIndex >= len(colors) {
					break
				}
				if !hiddenCol(row.HiddenCols, col) {
					x := chain.Bounds.X + row.Offset + col
					y := chain.Bounds.Y + rowIndex
					SetFrameColor(&frame, x, y, device.NewColor(colors[sendIndex]))
				}
				sendIndex++
			}
		}
	}
	return frame, nil
}

func rectangularMatrixFrameFromState(d device.Device, surface device.Surface, duration time.Duration) (Frame, error) {
	if len(d.MatrixProperties.ChainZones) == 0 || len(d.MatrixProperties.ChainZones[0]) == 0 {
		return Frame{}, fmt.Errorf("%w: missing matrix colors", ErrInvalidDeviceState)
	}
	frame := NewFrame(surface.Width, surface.Height, duration, blankColor)
	for i, color := range d.MatrixProperties.ChainZones[0] {
		if i >= len(frame.Colors) {
			break
		}
		frame.Colors[i] = device.NewColor(color)
	}
	return frame, nil
}

func matrixStateColors(colors []packets.LightHsbk, width, height int, orientation device.Orientation) []packets.LightHsbk {
	size := width * height
	out := make([]packets.LightHsbk, size)
	copy(out, colors)
	return device.ReorientMatrix(width, height, inverseOrientation(orientation), out)
}

func inverseOrientation(orientation device.Orientation) device.Orientation {
	switch orientation {
	case device.OrientationLeft:
		return device.OrientationRight
	case device.OrientationRight:
		return device.OrientationLeft
	default:
		return orientation
	}
}
