package adapters

import (
	"context"
	"errors"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/effects"
	"github.com/alessio-palumbo/lifxlan-go/pkg/messages"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/enums"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

var (
	// ErrEmptyFrame is returned when a frame contains no colors.
	ErrEmptyFrame = errors.New("empty frame")
	// ErrInvalidFrame is returned when a frame has invalid dimensions.
	ErrInvalidFrame = errors.New("invalid frame")
	// ErrMissingSendFunc is returned when a renderer has no send function.
	ErrMissingSendFunc = errors.New("missing send function")
)

// SendFunc sends a target-bound protocol message.
type SendFunc func(*protocol.Message) error

// NewRendererForDevice returns a renderer configured from device capabilities.
func NewRendererForDevice(d device.Device, send SendFunc) effects.Renderer {
	switch d.LightType {
	case device.LightTypeMultiZone:
		return NewMultiZoneRenderer(send, WithMultiZoneSurface(device.SurfaceFromDevice(d)))
	case device.LightTypeMatrix:
		surface := device.SurfaceFromDevice(d)
		length := 1
		if surface.Matrix != nil && len(surface.Matrix.Chains) > 0 {
			length = len(surface.Matrix.Chains)
		}
		opts := []MatrixOption{WithMatrixRange(0, length), WithMatrixSurface(surface)}
		return NewMatrixRenderer(send, opts...)
	default:
		return NewSingleZoneRenderer(send)
	}
}

// SingleZoneRenderer renders frames to a single-zone light.
type SingleZoneRenderer struct {
	send     SendFunc
	waveform enums.LightWaveform
}

// SingleZoneOption configures a SingleZoneRenderer.
type SingleZoneOption func(*SingleZoneRenderer)

// WithSingleZoneWaveform sets the waveform used for single-zone color commands.
func WithSingleZoneWaveform(waveform enums.LightWaveform) SingleZoneOption {
	return func(r *SingleZoneRenderer) {
		r.waveform = waveform
	}
}

// NewSingleZoneRenderer returns a single-zone renderer bound to send.
func NewSingleZoneRenderer(send SendFunc, opts ...SingleZoneOption) *SingleZoneRenderer {
	r := &SingleZoneRenderer{send: send}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// RenderFrame renders the first frame color as a single-zone color command.
func (r *SingleZoneRenderer) RenderFrame(ctx context.Context, frame effects.Frame) error {
	ctx, err := validateRenderer(ctx, r.send)
	if err != nil {
		return err
	}
	if len(frame.Colors) == 0 {
		return ErrEmptyFrame
	}

	color := frame.Colors[0]
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.send(messages.SetColor(&color.Hue, &color.Saturation, &color.Brightness, &color.Kelvin, frame.Duration, r.waveform))
}

// MultiZoneRenderer renders frames to a multi-zone light.
type MultiZoneRenderer struct {
	send       SendFunc
	startIndex int
	surface    *device.Surface
}

// MultiZoneOption configures a MultiZoneRenderer.
type MultiZoneOption func(*MultiZoneRenderer)

// WithMultiZoneStartIndex sets the first zone updated by multi-zone commands.
func WithMultiZoneStartIndex(startIndex int) MultiZoneOption {
	return func(r *MultiZoneRenderer) {
		r.startIndex = max(startIndex, 0)
	}
}

// WithMultiZoneSurface adapts logical frames to surface before sending.
func WithMultiZoneSurface(surface device.Surface) MultiZoneOption {
	return func(r *MultiZoneRenderer) {
		r.surface = &surface
	}
}

// NewMultiZoneRenderer returns a multi-zone renderer bound to send.
func NewMultiZoneRenderer(send SendFunc, opts ...MultiZoneOption) *MultiZoneRenderer {
	r := &MultiZoneRenderer{send: send}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// RenderFrame renders frame colors as multi-zone color commands.
func (r *MultiZoneRenderer) RenderFrame(ctx context.Context, frame effects.Frame) error {
	ctx, err := validateRenderer(ctx, r.send)
	if err != nil {
		return err
	}
	adaptFrame := multizoneFrame(frame)
	surface := r.multizoneSurface(adaptFrame)
	frames, err := effects.AdaptFrameToSurface(adaptFrame, surface, effects.AdaptOptions{})
	if err != nil {
		return mapEffectsError(err)
	}
	if len(frames) == 0 {
		return ErrEmptyFrame
	}
	colors, err := deviceFrameColors(frames[0])
	if err != nil {
		return err
	}
	return sendAll(ctx, r.send, messages.SetMultizoneExtendedColors(r.startIndex, colors, adaptFrame.Duration))
}

// MatrixRenderer renders frames to a matrix light.
type MatrixRenderer struct {
	send        SendFunc
	startIndex  int
	length      int
	orientation *device.Orientation
	surface     *device.Surface
}

// MatrixOption configures a MatrixRenderer.
type MatrixOption func(*MatrixRenderer)

// WithMatrixRange sets the tile range used for matrix commands.
func WithMatrixRange(startIndex, length int) MatrixOption {
	return func(r *MatrixRenderer) {
		r.startIndex = max(startIndex, 0)
		r.length = max(length, 1)
	}
}

// WithMatrixOrientation applies device orientation before sending matrix colors.
func WithMatrixOrientation(orientation device.Orientation) MatrixOption {
	return func(r *MatrixRenderer) {
		r.orientation = &orientation
	}
}

// WithMatrixSurface adapts logical frames to surface before sending.
func WithMatrixSurface(surface device.Surface) MatrixOption {
	return func(r *MatrixRenderer) {
		r.surface = &surface
		if surface.Matrix != nil && len(surface.Matrix.Chains) > 0 {
			r.length = len(surface.Matrix.Chains)
		}
	}
}

// NewMatrixRenderer returns a matrix renderer bound to send.
func NewMatrixRenderer(send SendFunc, opts ...MatrixOption) *MatrixRenderer {
	r := &MatrixRenderer{send: send, length: 1}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// RenderFrame renders frame colors as matrix color commands.
func (r *MatrixRenderer) RenderFrame(ctx context.Context, frame effects.Frame) error {
	ctx, err := validateRenderer(ctx, r.send)
	if err != nil {
		return err
	}
	if frame.Width <= 0 || frame.Height <= 0 {
		return ErrInvalidFrame
	}
	surface := r.renderSurface(frame)
	frames, err := effects.AdaptFrameToSurface(frame, surface, effects.AdaptOptions{})
	if err != nil {
		return mapEffectsError(err)
	}

	for _, deviceFrame := range frames {
		if err := ctx.Err(); err != nil {
			return err
		}
		colors, err := deviceFrameColors(deviceFrame)
		if err != nil {
			return err
		}
		orientation := deviceFrame.Orientation
		if r.orientation != nil {
			orientation = *r.orientation
		}
		colors = device.ReorientMatrix(deviceFrame.SendWidth, deviceFrame.Height, orientation, colors)
		startIndex := deviceFrame.ChainIndex
		if r.surface == nil {
			startIndex = r.startIndex
		}
		if err := sendAll(ctx, r.send, messages.SetMatrixColorsFromSlice(startIndex, r.length, deviceFrame.SendWidth, colors, deviceFrame.Duration)); err != nil {
			return err
		}
	}
	return nil
}

func validateRenderer(ctx context.Context, send SendFunc) (context.Context, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if send == nil {
		return nil, ErrMissingSendFunc
	}
	return ctx, nil
}

func (r *MultiZoneRenderer) multizoneSurface(frame effects.Frame) device.Surface {
	if r.surface != nil {
		return *r.surface
	}
	zones := len(frame.Colors)
	return device.Surface{
		LightType: device.LightTypeMultiZone,
		Width:     max(zones, 1),
		Height:    1,
		Zones:     max(zones, 1),
	}
}

func multizoneFrame(frame effects.Frame) effects.Frame {
	if frame.Width > 0 && frame.Height > 0 {
		return frame
	}
	frame.Width = max(len(frame.Colors), 1)
	frame.Height = 1
	return frame
}

func deviceFrameColors(frame effects.DeviceFrame) ([]packets.LightHsbk, error) {
	if len(frame.Colors) == 0 {
		return nil, ErrEmptyFrame
	}
	colors := make([]packets.LightHsbk, len(frame.Colors))
	for i, color := range frame.Colors {
		colors[i] = color.ToDeviceColor()
	}
	return colors, nil
}

func (r *MatrixRenderer) renderSurface(frame effects.Frame) device.Surface {
	if r.surface != nil {
		return *r.surface
	}

	orientation := device.OrientationRightSideUp
	if r.orientation != nil {
		orientation = *r.orientation
	}
	rows := make([]device.MatrixRow, frame.Height)
	for i := range rows {
		rows[i] = device.MatrixRow{Cols: frame.Width}
	}
	return device.Surface{
		LightType: device.LightTypeMatrix,
		Width:     frame.Width,
		Height:    frame.Height,
		Zones:     frame.Width * frame.Height,
		Matrix: &device.MatrixSurface{Chains: []device.MatrixChain{{
			Index:       r.startIndex,
			Bounds:      device.Rect{Width: frame.Width, Height: frame.Height},
			SendWidth:   frame.Width,
			Rows:        rows,
			Orientation: orientation,
		}}},
	}
}

func mapEffectsError(err error) error {
	switch {
	case errors.Is(err, effects.ErrEmptyFrame):
		return ErrEmptyFrame
	case errors.Is(err, effects.ErrInvalidFrame):
		return ErrInvalidFrame
	default:
		return err
	}
}

func sendAll(ctx context.Context, send SendFunc, msgs []*protocol.Message) error {
	for _, msg := range msgs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := send(msg); err != nil {
			return err
		}
	}
	return nil
}
