package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/controller"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

type partTestBackend struct {
	fakeBackend
	part                               device.LightPart
	serial                             device.Serial
	update                             controller.LightStateUpdate
	colorOptions                       controller.SnapshotOptions
	powerOptions                       controller.LightPartPowerOptions
	on                                 bool
	colorCalls, powerCalls, stateCalls int
	operationErr                       error
	state                              controller.LightPartState
}

func (b *partTestBackend) GetLightPartState(_ device.Serial, part device.LightPart) (controller.LightPartState, error) {
	b.stateCalls++
	s := b.state
	s.Part = part
	return s, nil
}
func (b *partTestBackend) SetLightPartColor(_ context.Context, serial device.Serial, part device.LightPart, update controller.LightStateUpdate, opts controller.SnapshotOptions) error {
	b.colorCalls++
	b.serial, b.part, b.update, b.colorOptions = serial, part, update, opts
	return b.operationErr
}
func (b *partTestBackend) SetLightPartPower(_ context.Context, serial device.Serial, part device.LightPart, on bool, opts controller.LightPartPowerOptions) error {
	b.powerCalls++
	b.serial, b.part, b.on, b.powerOptions = serial, part, on, opts
	return b.operationErr
}
func runPartTest(b *partTestBackend, args ...string) (string, error) {
	var out bytes.Buffer
	err := run(context.Background(), append([]string{"--discover-for", "1ns"}, args...), &out, io.Discard, func(bool, io.Writer) (backend, error) { return b, nil })
	return out.String(), err
}
func moonTestBackend() *partTestBackend {
	d := testLight()
	d.Label, d.ProductID, d.LightType = "Moon", 265, device.LightTypeMatrix
	d.MatrixProperties.Width, d.MatrixProperties.Height, d.MatrixProperties.ChainLength = 8, 8, 1
	return &partTestBackend{fakeBackend: fakeBackend{devices: []device.Device{d}}}
}

func TestPartColorForwardsPartialValuesAndFreshness(t *testing.T) {
	b := moonTestBackend()
	out, err := runPartTest(b, "devices", "color", "--target", "Moon", "--part", "uplight", "--brightness", "0", "--kelvin", "3500", "--duration", "200ms", "--timeout", "5s")
	if err != nil {
		t.Fatal(err)
	}
	if b.colorCalls != 1 || b.powerCalls != 0 || b.part != device.LightPartUplight || b.serial != b.devices[0].Serial || b.update.Brightness == nil || *b.update.Brightness != 0 || b.update.Hue != nil || b.update.Saturation != nil || b.update.Power != nil || *b.update.Kelvin != 3500 || b.update.Duration != 200*time.Millisecond || !b.colorOptions.RequireFresh || b.colorOptions.Timeout != 5*time.Second {
		t.Fatalf("incorrect forwarding: %+v", b)
	}
	if !strings.Contains(out, "verify observed state") || !b.closed {
		t.Fatal(out)
	}
}

func TestPartPowerForwardsIntentAndFallback(t *testing.T) {
	for _, intent := range []string{"on", "off"} {
		b := moonTestBackend()
		out, err := runPartTest(b, "devices", "power", "--target", "Moon", "--part", "main", "--duration", "100ms", "--timeout", "6s", "--fallback-brightness", "1", "--output", "json", intent)
		if err != nil {
			t.Fatal(err)
		}
		if b.powerCalls != 1 || b.colorCalls != 0 || b.on != (intent == "on") || b.part != device.LightPartMain || b.powerOptions.Timeout != 6*time.Second || b.powerOptions.Duration != 100*time.Millisecond || *b.powerOptions.FallbackBrightness != 1 || !strings.Contains(out, `"Part": "main"`) {
			t.Fatalf("incorrect forwarding: %+v, %s", b, out)
		}
	}
	b := moonTestBackend()
	if _, err := runPartTest(b, "devices", "power", "--target", "Moon", "on"); err != nil {
		t.Fatal(err)
	}
	if b.part != device.LightPartAll || b.powerOptions.FallbackBrightness != nil {
		t.Fatal("wrong defaults")
	}
}

func TestPartsFreshCachedAndUnknown(t *testing.T) {
	for _, cached := range []bool{false, true} {
		b := moonTestBackend()
		args := []string{"devices", "parts", "--target", "Moon"}
		if cached {
			args = append(args, "--fresh=false")
		}
		out, err := runPartTest(b, args...)
		if err != nil {
			t.Fatal(err)
		}
		if b.stateCalls != 2 || (b.captures == 1) == cached || !strings.Contains(out, "uplight") || !strings.Contains(out, "?") {
			t.Fatalf("unexpected inspection: %+v %s", b, out)
		}
		if !cached && !b.fresh {
			t.Fatal("capture not fresh")
		}
	}
	b := moonTestBackend()
	b.state = controller.LightPartState{Known: true, DevicePoweredOn: false, Brightness: 40, Emulated: true}
	out, err := runPartTest(b, "devices", "parts", "--target", "Moon", "--output", "json")
	if err != nil || !strings.Contains(out, `"Brightness": 40`) || !strings.Contains(out, `"DevicePoweredOn": false`) {
		t.Fatalf("%s %v", out, err)
	}
}

func TestInvalidPartCommandsStayOffline(t *testing.T) {
	for _, args := range [][]string{
		{"color", "--brightness", "101"}, {"color"}, {"color", "--kelvin", "3500.5"}, {"color", "--kelvin", "NaN"},
		{"color", "--brightness", "NaN"}, {"color", "--brightness", "1", "--part", "side"}, {"color", "--brightness", "1", "--duration", "-1s"},
		{"power", "toggle"}, {"power"}, {"power", "--fallback-brightness", "0", "on"}, {"power", "--fallback-brightness", "NaN", "on"},
		{"power", "--duration", "-1s", "off"}, {"parts", "extra"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			full := append([]string{"devices", args[0], "--target", "Moon"}, args[1:]...)
			err := run(context.Background(), full, io.Discard, io.Discard, func(bool, io.Writer) (backend, error) { t.Fatal("opened controller"); return nil, nil })
			if err == nil {
				t.Fatal("accepted invalid command")
			}
		})
	}
}

func TestPartCommandsRejectAmbiguityAndPropagateFailure(t *testing.T) {
	b := moonTestBackend()
	duplicate := b.devices[0]
	duplicate.Serial[5]++
	b.devices = append(b.devices, duplicate)
	if _, err := runPartTest(b, "devices", "power", "--target", "Moon", "off"); err == nil || b.powerCalls != 0 {
		t.Fatal("ambiguous write accepted")
	}
	b = moonTestBackend()
	b.operationErr = controller.ErrNoInheritedBrightness
	if _, err := runPartTest(b, "devices", "power", "--target", "Moon", "on"); !errors.Is(err, controller.ErrNoInheritedBrightness) {
		t.Fatal(err)
	}
	b = moonTestBackend()
	b.captureErr = context.DeadlineExceeded
	if _, err := runPartTest(b, "devices", "parts", "--target", "Moon"); !errors.Is(err, context.DeadlineExceeded) || b.stateCalls != 0 {
		t.Fatal(err)
	}
}
