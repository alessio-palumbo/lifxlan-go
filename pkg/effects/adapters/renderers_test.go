package adapters

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/effects"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/enums"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func TestSingleZoneRendererSendsColorMessage(t *testing.T) {
	sender := &recordingSender{}
	renderer := NewSingleZoneRenderer(sender.Send)

	err := renderer.RenderFrame(context.Background(), effects.Frame{
		Colors:   []effects.Color{{Hue: 180, Saturation: 100, Brightness: 50, Kelvin: 3500}},
		Duration: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sender.messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(sender.messages))
	}

	payload := sender.messages[0].Payload.(*packets.LightSetWaveformOptional)
	if !payload.SetHue || !payload.SetSaturation || !payload.SetBrightness || !payload.SetKelvin {
		t.Fatal("expected all color fields to be set")
	}
	if payload.Color.Hue != 32768 || payload.Color.Saturation != math.MaxUint16 ||
		payload.Color.Brightness != 32768 || payload.Color.Kelvin != 3500 {
		t.Fatalf("color = %#v", payload.Color)
	}
	if payload.Period != 1000 {
		t.Fatalf("period = %d, want 1000", payload.Period)
	}
}

func TestNewRendererForDeviceSelectsRenderer(t *testing.T) {
	tests := map[string]struct {
		device device.Device
		want   any
	}{
		"single zone": {
			device: device.Device{LightType: device.LightTypeSingleZone},
			want:   &SingleZoneRenderer{},
		},
		"multi zone": {
			device: device.Device{LightType: device.LightTypeMultiZone},
			want:   &MultiZoneRenderer{},
		},
		"matrix": {
			device: device.Device{LightType: device.LightTypeMatrix},
			want:   &MatrixRenderer{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := NewRendererForDevice(tt.device, (&recordingSender{}).Send)
			if reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Fatalf("renderer = %T, want %T", got, tt.want)
			}
		})
	}
}

func TestNewRendererForDeviceConfiguresMatrix(t *testing.T) {
	sender := &recordingSender{}
	renderer := NewRendererForDevice(device.Device{
		LightType: device.LightTypeMatrix,
		MatrixProperties: device.MatrixProperties{
			Width:             2,
			Height:            2,
			NZones:            4,
			ChainLength:       1,
			ChainOrientations: []device.Orientation{device.OrientationUpsideDown},
		},
	}, sender.Send)

	err := renderer.RenderFrame(context.Background(), effects.Frame{
		Colors: []effects.Color{kelvinColor(3500), kelvinColor(3600), kelvinColor(3700), kelvinColor(3800)},
		Width:  2,
		Height: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	payload := sender.messages[0].Payload.(*packets.TileSet64)
	if payload.TileIndex != 0 || payload.Length != 1 {
		t.Fatalf("matrix range = index %d length %d, want 0/1", payload.TileIndex, payload.Length)
	}
	if got := payload.Colors[0].Kelvin; got != 3800 {
		t.Fatalf("first kelvin = %d, want 3800 after orientation", got)
	}
}

func TestSingleZoneRendererUsesWaveformOption(t *testing.T) {
	sender := &recordingSender{}
	renderer := NewSingleZoneRenderer(sender.Send, WithSingleZoneWaveform(enums.LightWaveformLIGHTWAVEFORMPULSE))

	err := renderer.RenderFrame(context.Background(), effects.Frame{Colors: []effects.Color{kelvinColor(3500)}})
	if err != nil {
		t.Fatal(err)
	}

	payload := sender.messages[0].Payload.(*packets.LightSetWaveformOptional)
	if payload.Waveform != enums.LightWaveformLIGHTWAVEFORMPULSE {
		t.Fatalf("waveform = %v, want pulse", payload.Waveform)
	}
}

func TestMultiZoneRendererSendsExtendedZoneMessages(t *testing.T) {
	sender := &recordingSender{}
	renderer := NewMultiZoneRenderer(sender.Send, WithMultiZoneStartIndex(5))

	err := renderer.RenderFrame(context.Background(), effects.Frame{
		Colors:   []effects.Color{kelvinColor(3500), kelvinColor(3600)},
		Duration: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sender.messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(sender.messages))
	}

	payload := sender.messages[0].Payload.(*packets.MultiZoneExtendedSetColorZones)
	if payload.Index != 5 {
		t.Fatalf("index = %d, want 5", payload.Index)
	}
	if payload.ColorsCount != 2 {
		t.Fatalf("colors count = %d, want 2", payload.ColorsCount)
	}
	if payload.Duration != 2000 {
		t.Fatalf("duration = %d, want 2000", payload.Duration)
	}
	if payload.Colors[0].Kelvin != 3500 || payload.Colors[1].Kelvin != 3600 {
		t.Fatalf("colors = %#v", payload.Colors[:2])
	}
}

func TestMultiZoneRendererSendsMultipleMessages(t *testing.T) {
	sender := &recordingSender{}
	renderer := NewMultiZoneRenderer(sender.Send)
	colors := make([]effects.Color, 83)
	for i := range colors {
		colors[i] = kelvinColor(uint16(3000 + i))
	}

	err := renderer.RenderFrame(context.Background(), effects.Frame{Colors: colors})
	if err != nil {
		t.Fatal(err)
	}
	if len(sender.messages) != 3 {
		t.Fatalf("messages = %d, want 3", len(sender.messages))
	}
}

func TestMultiZoneRendererUsesSurfaceAdaptation(t *testing.T) {
	sender := &recordingSender{}
	surface := device.Surface{
		LightType: device.LightTypeMultiZone,
		Width:     2,
		Height:    1,
		Zones:     2,
	}
	renderer := NewMultiZoneRenderer(sender.Send, WithMultiZoneSurface(surface))

	err := renderer.RenderFrame(context.Background(), effects.Frame{
		Colors: []effects.Color{kelvinColor(3500), kelvinColor(3600), kelvinColor(3700), kelvinColor(3800)},
		Width:  4,
		Height: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	payload := sender.messages[0].Payload.(*packets.MultiZoneExtendedSetColorZones)
	if payload.ColorsCount != 2 {
		t.Fatalf("colors count = %d, want 2", payload.ColorsCount)
	}
	if payload.Colors[0].Kelvin != 3500 || payload.Colors[1].Kelvin != 3700 {
		t.Fatalf("colors = %#v", payload.Colors[:2])
	}
}

func TestNewRendererForDeviceConfiguresMultiZoneSurface(t *testing.T) {
	sender := &recordingSender{}
	renderer := NewRendererForDevice(device.Device{
		LightType: device.LightTypeMultiZone,
		MultizoneProperties: device.MultizoneProperties{
			Zones: make([]packets.LightHsbk, 2),
		},
	}, sender.Send)

	err := renderer.RenderFrame(context.Background(), effects.Frame{
		Colors: []effects.Color{kelvinColor(3500), kelvinColor(3600), kelvinColor(3700), kelvinColor(3800)},
		Width:  4,
		Height: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	payload := sender.messages[0].Payload.(*packets.MultiZoneExtendedSetColorZones)
	if payload.ColorsCount != 2 {
		t.Fatalf("colors count = %d, want 2", payload.ColorsCount)
	}
	if payload.Colors[0].Kelvin != 3500 || payload.Colors[1].Kelvin != 3700 {
		t.Fatalf("colors = %#v", payload.Colors[:2])
	}
}

func TestMatrixRendererSendsTileMessages(t *testing.T) {
	sender := &recordingSender{}
	renderer := NewMatrixRenderer(sender.Send, WithMatrixRange(1, 2))

	err := renderer.RenderFrame(context.Background(), effects.Frame{
		Colors:   []effects.Color{kelvinColor(3500), kelvinColor(3600), kelvinColor(3700), kelvinColor(3800)},
		Width:    2,
		Height:   2,
		Duration: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sender.messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(sender.messages))
	}

	payload := sender.messages[0].Payload.(*packets.TileSet64)
	if payload.TileIndex != 1 || payload.Length != 2 || payload.Rect.Width != 2 {
		t.Fatalf("payload range = index %d length %d width %d", payload.TileIndex, payload.Length, payload.Rect.Width)
	}
	if payload.Duration != 1000 {
		t.Fatalf("duration = %d, want 1000", payload.Duration)
	}
	if payload.Colors[0].Kelvin != 3500 || payload.Colors[3].Kelvin != 3800 {
		t.Fatalf("colors = %#v", payload.Colors[:4])
	}
}

func TestMatrixRendererAppliesOrientation(t *testing.T) {
	sender := &recordingSender{}
	renderer := NewMatrixRenderer(sender.Send, WithMatrixOrientation(device.OrientationUpsideDown))

	err := renderer.RenderFrame(context.Background(), effects.Frame{
		Colors: []effects.Color{kelvinColor(3500), kelvinColor(3600), kelvinColor(3700), kelvinColor(3800)},
		Width:  2,
		Height: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	payload := sender.messages[0].Payload.(*packets.TileSet64)
	got := []uint16{payload.Colors[0].Kelvin, payload.Colors[1].Kelvin, payload.Colors[2].Kelvin, payload.Colors[3].Kelvin}
	want := []uint16{3800, 3700, 3600, 3500}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("kelvins = %v, want %v", got, want)
		}
	}
}

func TestMatrixRendererUsesSurfaceDeviceFrames(t *testing.T) {
	sender := &recordingSender{}
	surface := device.SurfaceFromDevice(device.Device{
		LightType: device.LightTypeMatrix,
		MatrixProperties: device.MatrixProperties{
			Width:       2,
			Height:      1,
			NZones:      2,
			ChainLength: 2,
			ChainOrientations: []device.Orientation{
				device.OrientationRightSideUp,
				device.OrientationRightSideUp,
			},
		},
	})
	renderer := NewMatrixRenderer(sender.Send, WithMatrixSurface(surface))

	err := renderer.RenderFrame(context.Background(), effects.Frame{
		Colors: []effects.Color{kelvinColor(3500), kelvinColor(3600), kelvinColor(3700), kelvinColor(3800)},
		Width:  4,
		Height: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sender.messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(sender.messages))
	}

	first := sender.messages[0].Payload.(*packets.TileSet64)
	second := sender.messages[1].Payload.(*packets.TileSet64)
	if first.TileIndex != 0 || first.Length != 2 || first.Rect.Width != 2 {
		t.Fatalf("first payload = index %d length %d width %d", first.TileIndex, first.Length, first.Rect.Width)
	}
	if second.TileIndex != 1 || second.Length != 2 || second.Rect.Width != 2 {
		t.Fatalf("second payload = index %d length %d width %d", second.TileIndex, second.Length, second.Rect.Width)
	}
	if first.Colors[0].Kelvin != 3500 || first.Colors[1].Kelvin != 3600 {
		t.Fatalf("first colors = %#v", first.Colors[:2])
	}
	if second.Colors[0].Kelvin != 3700 || second.Colors[1].Kelvin != 3800 {
		t.Fatalf("second colors = %#v", second.Colors[:2])
	}
}

func TestMatrixRendererUsesSurfaceSendWidthAndHiddenCells(t *testing.T) {
	sender := &recordingSender{}
	surface := device.SurfaceFromDevice(device.Device{
		ProductID: 201,
		LightType: device.LightTypeMatrix,
		MatrixProperties: device.MatrixProperties{
			Width:      8,
			Height:     16,
			NZones:     128,
			ChainZones: [][]packets.LightHsbk{make([]packets.LightHsbk, 128)},
		},
	})
	renderer := NewMatrixRenderer(sender.Send, WithMatrixSurface(surface))

	err := renderer.RenderFrame(context.Background(), indexedFrame(16, 8, time.Second))
	if err != nil {
		t.Fatal(err)
	}

	payload := sender.messages[0].Payload.(*packets.TileSet64)
	if payload.Rect.Width != 8 {
		t.Fatalf("send width = %d, want 8", payload.Rect.Width)
	}
	if payload.Colors[0] != (packets.LightHsbk{}) || payload.Colors[1] != (packets.LightHsbk{}) {
		t.Fatalf("hidden colors = %#v", payload.Colors[:2])
	}
	if payload.Colors[2].Kelvin != 3502 {
		t.Fatalf("first visible kelvin = %d, want 3502", payload.Colors[2].Kelvin)
	}
}

func TestRendererValidation(t *testing.T) {
	tests := map[string]struct {
		render func() error
		want   error
	}{
		"missing send": {
			render: func() error {
				return NewSingleZoneRenderer(nil).RenderFrame(context.Background(), effects.Frame{Colors: []effects.Color{kelvinColor(3500)}})
			},
			want: ErrMissingSendFunc,
		},
		"empty frame": {
			render: func() error {
				return NewSingleZoneRenderer((&recordingSender{}).Send).RenderFrame(context.Background(), effects.Frame{})
			},
			want: ErrEmptyFrame,
		},
		"invalid matrix frame": {
			render: func() error {
				return NewMatrixRenderer((&recordingSender{}).Send).RenderFrame(context.Background(), effects.Frame{Colors: []effects.Color{kelvinColor(3500)}})
			},
			want: ErrInvalidFrame,
		},
		"canceled context": {
			render: func() error {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return NewSingleZoneRenderer((&recordingSender{}).Send).RenderFrame(ctx, effects.Frame{Colors: []effects.Color{kelvinColor(3500)}})
			},
			want: context.Canceled,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := tt.render()
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

type recordingSender struct {
	messages []*protocol.Message
}

func (s *recordingSender) Send(msg *protocol.Message) error {
	s.messages = append(s.messages, msg)
	return nil
}

func kelvinColor(kelvin uint16) effects.Color {
	return effects.Color{Brightness: 100, Kelvin: kelvin}
}

func indexedFrame(width, height int, duration time.Duration) effects.Frame {
	colors := make([]effects.Color, width*height)
	for i := range colors {
		colors[i] = kelvinColor(uint16(3500 + i))
	}
	return effects.Frame{
		Colors:   colors,
		Width:    width,
		Height:   height,
		Duration: duration,
	}
}
