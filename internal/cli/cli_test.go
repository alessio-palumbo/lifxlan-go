package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/controller"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/effects"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
)

type fakeBackend struct {
	devices                         []device.Device
	sends, captures, restores       int
	fresh                           bool
	captureErr, restoreErr, sendErr error
	onSend                          func()
	closed                          bool
}

func (f *fakeBackend) Close() error                { f.closed = true; return nil }
func (f *fakeBackend) GetDevices() []device.Device { return f.devices }
func (f *fakeBackend) Send(device.Serial, *protocol.Message) error {
	f.sends++
	if f.onSend != nil {
		f.onSend()
	}
	return f.sendErr
}
func (f *fakeBackend) Ping(context.Context, device.Serial) (time.Duration, error) {
	return time.Millisecond, nil
}
func (f *fakeBackend) SubscribeDevices(context.Context, ...controller.SubscriptionOption) <-chan controller.DeviceEvent {
	ch := make(chan controller.DeviceEvent)
	close(ch)
	return ch
}
func (f *fakeBackend) CaptureStateSnapshot(_ context.Context, serials []device.Serial, opts controller.SnapshotOptions) (device.StateSnapshot, error) {
	f.captures++
	f.fresh = opts.RequireFresh
	return device.NewStateSnapshot(f.devices), f.captureErr
}
func (f *fakeBackend) RestoreStateSnapshot(ctx context.Context, _ device.StateSnapshot, _ controller.RestoreOptions) error {
	f.restores++
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return f.restoreErr
}

func testLight() device.Device {
	serial, _ := device.SerialFromHex("d073d5000001")
	return device.Device{Serial: serial, Label: "Desk", Type: device.DeviceTypeLight, LightType: device.LightTypeSingleZone, PoweredOn: true}
}
func factoryFor(f *fakeBackend) factory {
	return func(bool, io.Writer) (backend, error) { return f, nil }
}

func TestOfflineAndInvalidCommandsDoNotOpenController(t *testing.T) {
	for _, args := range [][]string{
		{}, {"help"}, {"effects", "list"}, {"ping"}, {"ping", "--target", "Desk", "--count", "0"},
		{"effects", "run", "--target", "Desk", "--step", "1ms", "breathe"},
		{"effects", "run", "--target", "Desk", "unknown"}, {"devices", "extra"}, {"unknown"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out bytes.Buffer
			_ = run(context.Background(), args, &out, io.Discard, func(bool, io.Writer) (backend, error) { t.Fatal("opened controller"); return nil, nil })
		})
	}
}

func TestTargetSelection(t *testing.T) {
	d := testLight()
	if got, err := selectDevice([]device.Device{d}, "desk"); err != nil || got.Serial != d.Serial {
		t.Fatalf("got %v, %v", got, err)
	}
	if _, err := selectDevice([]device.Device{d, d}, "Desk"); err == nil {
		t.Fatal("accepted duplicate label")
	}
	if _, err := selectDevice([]device.Device{d}, "missing"); err == nil {
		t.Fatal("accepted missing target")
	}
	if got, err := selectDevice([]device.Device{d}, strings.ToUpper(d.Serial.String())); err != nil || got.Serial != d.Serial {
		t.Fatal("serial lookup failed")
	}
}

func TestCommandDryRunAndDispatch(t *testing.T) {
	for _, dry := range []bool{true, false} {
		f := &fakeBackend{devices: []device.Device{testLight()}}
		var out bytes.Buffer
		args := []string{"command", "--discover-for", "1ns"}
		if dry {
			args = append(args, "--dry-run")
		}
		args = append(args, "Desk blue 50%")
		if err := run(context.Background(), args, &out, io.Discard, factoryFor(f)); err != nil {
			t.Fatal(err)
		}
		if dry && f.sends != 0 {
			t.Fatal("dry-run sent control messages")
		}
		if !dry && f.sends == 0 {
			t.Fatal("no dispatch")
		}
		if !strings.Contains(out.String(), "Targets") || !strings.Contains(out.String(), "Payload") {
			t.Fatal("missing plan")
		}
		if !f.closed {
			t.Fatal("controller not closed")
		}
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestCommandOutputFailurePreventsSend(t *testing.T) {
	f := &fakeBackend{devices: []device.Device{testLight()}}
	err := executeCommand(context.Background(), f, f.devices, "Desk blue", false, brokenWriter{})
	if !errors.Is(err, io.ErrClosedPipe) || f.sends != 0 {
		t.Fatalf("err=%v sends=%d", err, f.sends)
	}
}

func TestEffectCaptureFailurePreventsSend(t *testing.T) {
	f := &fakeBackend{devices: []device.Device{testLight()}, captureErr: errors.New("incomplete pixels")}
	err := runEffect(context.Background(), f, testLight(), effects.Config{ID: effects.EffectBreathe}, time.Millisecond, 0, time.Second, true)
	if err == nil || f.sends != 0 || f.restores != 0 || !f.fresh {
		t.Fatalf("err=%v fake=%+v", err, f)
	}
}

func TestEffectCancellationRestoresWithLiveContext(t *testing.T) {
	for _, restoreFails := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		f := &fakeBackend{devices: []device.Device{testLight()}, onSend: cancel}
		failure := errors.New("restore failed")
		if restoreFails {
			f.restoreErr = failure
		}
		err := runEffect(ctx, f, testLight(), effects.Config{ID: effects.EffectBreathe}, time.Millisecond, 0, time.Second, true)
		cancel()
		if f.restores != 1 || f.sends != 1 {
			t.Fatalf("fake=%+v", f)
		}
		if restoreFails {
			if !errors.Is(err, failure) || errors.Is(err, context.Canceled) {
				t.Fatalf("restore failure hidden: %v", err)
			}
		} else if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v", err)
		}
	}
}

func TestEffectSendErrorAndNoRestoreOption(t *testing.T) {
	for _, restore := range []bool{true, false} {
		failure := errors.New("send failure")
		f := &fakeBackend{devices: []device.Device{testLight()}, sendErr: failure}
		err := runEffect(context.Background(), f, testLight(), effects.Config{ID: effects.EffectBreathe}, time.Millisecond, 0, time.Second, restore)
		want := 0
		if restore {
			want = 1
		}
		if !errors.Is(err, failure) || f.restores != want {
			t.Fatalf("err=%v restores=%d", err, f.restores)
		}
	}
}

func TestCanceledDiscoveryClosesController(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	f := &fakeBackend{}
	err := run(ctx, []string{"devices"}, io.Discard, io.Discard, func(bool, io.Writer) (backend, error) { cancel(); return f, nil })
	if !errors.Is(err, context.Canceled) || !f.closed {
		t.Fatalf("err=%v closed=%v", err, f.closed)
	}
}

func TestSnapshotFreshAndInspect(t *testing.T) {
	for _, name := range []string{"snapshot", "inspect", "ping"} {
		f := &fakeBackend{devices: []device.Device{testLight()}}
		args := []string{name, "--discover-for", "1ns", "--target", "Desk"}
		if name == "snapshot" {
			args = append(args, "--fresh")
		}
		var out bytes.Buffer
		if err := run(context.Background(), args, &out, io.Discard, factoryFor(f)); err != nil {
			t.Fatal(err)
		}
		if out.Len() == 0 {
			t.Fatal("no output")
		}
		if name == "snapshot" && (!f.fresh || f.captures != 1) {
			t.Fatal("freshness missing")
		}
	}
}

func TestEffectDurationRestores(t *testing.T) {
	f := &fakeBackend{devices: []device.Device{testLight()}}
	err := runEffect(context.Background(), f, testLight(), effects.Config{ID: effects.EffectColorCycle}, time.Millisecond, 5*time.Millisecond, time.Second, true)
	if err != nil || f.restores != 1 || f.sends == 0 {
		t.Fatalf("err=%v fake=%+v", err, f)
	}
}

func TestEffectConfigErrorsDoNotOpenController(t *testing.T) {
	for _, value := range []string{`{"id":"solid"}`, `{"id":"breathe","unknown":1}`, `{"id":"breathe"} {}`, `invalid`} {
		path := filepath.Join(t.TempDir(), "effect.json")
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
		err := run(context.Background(), []string{"effects", "run", "--target", "Desk", "--config", path, "breathe"}, io.Discard, io.Discard, func(bool, io.Writer) (backend, error) {
			t.Fatal("opened controller for invalid config")
			return nil, nil
		})
		if err == nil {
			t.Fatalf("accepted %s", value)
		}
	}
}

func TestDeviceOutputHasCopyableSerial(t *testing.T) {
	f := &fakeBackend{devices: []device.Device{testLight()}}
	var out bytes.Buffer
	if err := run(context.Background(), []string{"devices", "--discover-for", "1ns"}, &out, io.Discard, factoryFor(f)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"Serial": "d073d5000001"`) {
		t.Fatalf("serial is not copyable: %s", out.String())
	}
}
