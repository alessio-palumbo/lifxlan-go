package adapters

import (
	"context"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/effects"
)

func TestRunEffectsUsesDeviceRenderer(t *testing.T) {
	sender := &recordingSender{}

	err := RunEffects(context.Background(), device.Device{
		LightType: device.LightTypeMultiZone,
	}, sender.Send, effects.RunConfig{
		Effect: &oneFrameEffect{frame: effects.Frame{
			Colors: []effects.Color{kelvinColor(3500), kelvinColor(3500)},
			Width:  2,
			Height: 1,
		}},
		Step: time.Nanosecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sender.messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(sender.messages))
	}
}

type oneFrameEffect struct {
	frame effects.Frame
	done  bool
}

func (e *oneFrameEffect) Next(dt time.Duration) (effects.Frame, bool) {
	if e.done {
		return effects.Frame{}, false
	}
	e.done = true
	return e.frame, true
}

func (e *oneFrameEffect) Reset() {
	e.done = false
}
