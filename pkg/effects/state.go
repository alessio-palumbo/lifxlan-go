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
	return adaptPhysicalColorStateToFrame(
		PhysicalColorState{Zones: d.MultizoneProperties.Zones},
		device.SurfaceFromDevice(d),
		duration,
		packets.LightHsbk{},
		device.NewColor,
	)
}

func frameFromMatrixState(d device.Device, duration time.Duration) (Frame, error) {
	surface := device.SurfaceFromDevice(d)
	if surface.Width <= 0 || surface.Height <= 0 {
		return Frame{}, fmt.Errorf("%w: invalid matrix surface", ErrInvalidDeviceState)
	}
	if len(d.MatrixProperties.ChainZones) == 0 {
		return Frame{}, fmt.Errorf("%w: missing matrix colors", ErrInvalidDeviceState)
	}
	if surface.Matrix != nil {
		for _, chain := range surface.Matrix.Chains {
			if chain.Index < 0 || chain.Index >= len(d.MatrixProperties.ChainZones) || len(d.MatrixProperties.ChainZones[chain.Index]) == 0 {
				return Frame{}, fmt.Errorf("%w: missing matrix chain %d colors", ErrInvalidDeviceState, chain.Index)
			}
		}
	}
	return adaptPhysicalColorStateToFrame(
		PhysicalColorState{MatrixChains: d.MatrixProperties.ChainZones},
		surface,
		duration,
		packets.LightHsbk{},
		device.NewColor,
	)
}
