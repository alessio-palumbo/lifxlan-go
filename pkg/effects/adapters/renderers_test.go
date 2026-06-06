package adapters

import (
	"context"
	"errors"
	"math"
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
