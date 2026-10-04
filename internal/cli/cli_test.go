package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/controller"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/effects"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

type fakeBackend struct {
	devices                         []device.Device
	sends, captures, restores       int
	fresh                           bool
	captureErr, restoreErr, sendErr error
	onSend                          func()
	closed                          bool
	pingCalls                       int
	pingErrors                      []error
	onGetDevices                    func() []device.Device
	events                          []controller.DeviceEvent
	subscriptions                   int
}

func (f *fakeBackend) Close() error { f.closed = true; return nil }
func (f *fakeBackend) GetDevices() []device.Device {
	if f.onGetDevices != nil {
		return f.onGetDevices()
	}
	return f.devices
}
func (f *fakeBackend) Send(device.Serial, *protocol.Message) error {
	f.sends++
	if f.onSend != nil {
		f.onSend()
	}
	return f.sendErr
}
func (f *fakeBackend) Ping(context.Context, device.Serial) (time.Duration, error) {
	index := f.pingCalls
	f.pingCalls++
	if index < len(f.pingErrors) && f.pingErrors[index] != nil {
		return 0, f.pingErrors[index]
	}
	return time.Millisecond, nil
}
func (f *fakeBackend) SubscribeDevices(context.Context, ...controller.SubscriptionOption) <-chan controller.DeviceEvent {
	f.subscriptions++
	ch := make(chan controller.DeviceEvent, len(f.events))
	for _, event := range f.events {
		ch <- event
	}
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
		{}, {"help"}, {"effects", "list"}, {"devices", "ping"}, {"devices", "ping", "--target", "Desk", "--count", "0"},
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
		args := []string{"command", "--output", "json", "--discover-for", "1ns"}
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
	err := executeCommand(context.Background(), f, f.devices, "Desk blue", false, brokenWriter{}, "text")
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
		if name != "snapshot" {
			args = append([]string{"devices"}, args...)
		}
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
	if err := run(context.Background(), []string{"devices", "--output", "json", "--discover-for", "1ns"}, &out, io.Discard, factoryFor(f)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"Serial": "d073d5000001"`) {
		t.Fatalf("serial is not copyable: %s", out.String())
	}
}

func TestHumanOutputDefaults(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"devices", "--discover-for", "1ns"}, "SERIAL"},
		{[]string{"devices", "inspect", "--discover-for", "1ns", "--target", "Desk"}, "Estimated uptime"},
		{[]string{"devices", "ping", "--discover-for", "1ns", "--target", "Desk", "--count", "2"}, "2 sent, 2 received, 0.0% loss"},
		{[]string{"command", "--discover-for", "1ns", "--dry-run", "Desk blue"}, "Dry-run plan"},
		{[]string{"effects", "list"}, "Defaults:"},
	} {
		f := &fakeBackend{devices: []device.Device{testLight()}}
		var out bytes.Buffer
		if err := run(context.Background(), test.args, &out, io.Discard, factoryFor(f)); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), test.want) {
			t.Fatalf("%v: %s", test.args, out.String())
		}
	}
}

func TestNestedHelpAndInvalidOutputAreOffline(t *testing.T) {
	for _, args := range [][]string{{"effects", "--help"}, {"effects", "run", "--help"}, {"help", "effects", "run"}, {"devices", "--output", "yaml"}} {
		var out bytes.Buffer
		err := run(context.Background(), args, &out, io.Discard, func(bool, io.Writer) (backend, error) { t.Fatal("opened controller"); return nil, nil })
		if args[0] == "devices" {
			if err == nil {
				t.Fatal("accepted invalid output")
			}
		} else if err != nil {
			t.Fatal(err)
		}
		if args[0] == "effects" && !strings.Contains(out.String(), "USAGE:") {
			t.Fatal("missing contextual help")
		}
	}
}

func TestInheritedOutputAndSnapshotJSON(t *testing.T) {
	for _, args := range [][]string{{"--output", "json", "effects", "list"}, {"snapshot", "--discover-for", "1ns", "--target", "Desk"}} {
		var out bytes.Buffer
		f := &fakeBackend{devices: []device.Device{testLight()}}
		if err := run(context.Background(), args, &out, io.Discard, factoryFor(f)); err != nil {
			t.Fatal(err)
		}
		if !json.Valid(out.Bytes()) {
			t.Fatalf("not clean JSON: %s", out.String())
		}
	}
}

func TestMixedPingLossAndJSONSamples(t *testing.T) {
	for _, format := range []string{"text", "json"} {
		failure := errors.New("timeout")
		f := &fakeBackend{devices: []device.Device{testLight()}, pingErrors: []error{nil, failure, nil}}
		var out bytes.Buffer
		err := run(context.Background(), []string{"devices", "ping", "--target", "Desk", "--discover-for", "1ns", "--count", "3", "--output", format}, &out, io.Discard, factoryFor(f))
		if !errors.Is(err, failure) || f.pingCalls != 3 {
			t.Fatalf("err=%v calls=%d", err, f.pingCalls)
		}
		if format == "text" {
			if !strings.Contains(out.String(), "3 sent, 2 received, 33.3% loss") || !strings.Contains(out.String(), "RTT min/avg/max") {
				t.Fatal(out.String())
			}
		} else {
			lines := strings.Split(strings.TrimSpace(out.String()), "\n")
			if len(lines) != 3 {
				t.Fatal(out.String())
			}
			for _, line := range lines {
				if !json.Valid([]byte(line)) {
					t.Fatal(line)
				}
			}
		}
	}
}

func TestHumanLabelsCannotControlTerminal(t *testing.T) {
	d := testLight()
	d.Label = "Desk\x1b[2J\n\t"
	d.Group = "Office\r\n"
	var out bytes.Buffer
	if err := printDevices(&out, []device.Device{d}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\x1b") || strings.Contains(out.String(), "Desk\n") || strings.Contains(out.String(), "Office\r") {
		t.Fatal("unsanitised labels")
	}
}

func TestHumanInventoryEventsHaveNoFakeDevice(t *testing.T) {
	for _, eventType := range []controller.DeviceEventType{controller.DeviceEventSnapshotComplete, controller.DeviceEventResyncRequired} {
		var out bytes.Buffer
		if err := printEvent(&out, controller.DeviceEvent{Type: eventType, Revision: 7}); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "000000000000") || !strings.Contains(out.String(), eventType.String()) {
			t.Fatal(out.String())
		}
	}
}

func TestDeviceColumnOrderAndMetadata(t *testing.T) {
	d := testLight()
	d.Location = "Home"
	d.Group = "Office"
	d.ProductID = 27
	d.FirmwareVersion = "3.90"
	d.WifiRSSI = -42
	d.Address = &net.UDPAddr{IP: net.ParseIP("192.168.1.42"), Port: 56700}
	d.EstimatedBootedAt = time.Now().Add(-2 * time.Hour)
	var out bytes.Buffer
	if err := printDevices(&out, []device.Device{d}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	want := []string{"SERIAL", "IP", "LABEL", "LOCATION", "GROUP", "TYPE", "ZONES", "PRODUCT_ID", "FIRMWARE", "POWER", "RSSI/SNR", "UPTIME"}
	got := strings.Fields(lines[0])
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("columns=%v", got)
	}
	row := strings.Fields(lines[1])
	if len(row) != 12 || row[1] != "192.168.1.42" || row[3] != "Home" || row[4] != "Office" || row[6] != "1" || row[7] != "27" || row[8] != "3.90" || row[10] != "-42" || !strings.HasPrefix(row[11], "2h") {
		t.Fatalf("row=%v", row)
	}
	if firmware(testLight()) != "-" || wifiSignal(testLight()) != "-" {
		t.Fatal("missing metadata displayed as values")
	}
	d.WifiRSSI = 35
	if wifiSignal(d) != "35" {
		t.Fatal("positive SNR not preserved")
	}
}

func TestInventoryTypeCountsAndMissingAddressUptime(t *testing.T) {
	d := testLight()
	if ipAddress(d) != "-" || estimatedUptime(d) != "-" {
		t.Fatal("missing address/uptime not marked")
	}
	d.LightType = device.LightTypeMultiZone
	if kind(d) != "multi_zone" || zones(d) != "?" {
		t.Fatal(kind(d), zones(d))
	}
	d.MultizoneProperties.Zones = make([]packets.LightHsbk, 34)
	if zones(d) != "34" {
		t.Fatal(zones(d))
	}
	d.LightType = device.LightTypeMatrix
	if kind(d) != "matrix" || zones(d) != "?" {
		t.Fatal(kind(d), zones(d))
	}
	d.MatrixProperties.Width = 8
	d.MatrixProperties.Height = 8
	d.MatrixProperties.ChainLength = 1
	if zones(d) != "64" {
		t.Fatal(zones(d))
	}
	d.MatrixProperties.ChainLength = 5
	if zones(d) != "320" {
		t.Fatal(zones(d))
	}
	d.Type = device.DeviceTypeSwitch
	if kind(d) != d.Type.String() || zones(d) != "-" {
		t.Fatal("switch has pixel count")
	}
}

func TestJSONZoneCountNumericOrNull(t *testing.T) {
	d := testLight()
	for _, test := range []struct {
		product uint32
		want    string
	}{{0, `"ZoneCount": null`}, {27, `"ZoneCount": 1`}} {
		d.ProductID = test.product
		var out bytes.Buffer
		if err := writeJSON(&out, viewDevice(d)); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), test.want) {
			t.Fatal(out.String())
		}
	}
	d.LightType = device.LightTypeMatrix
	d.MatrixProperties.Width = 8
	d.MatrixProperties.Height = 8
	d.MatrixProperties.ChainLength = 5
	if count := viewDevice(d).ZoneCount; count == nil || *count != 320 {
		t.Fatal(count)
	}
	d.Type = device.DeviceTypeSwitch
	if viewDevice(d).ZoneCount != nil {
		t.Fatal("switch has a zone count")
	}
}
