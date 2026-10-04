package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

type cancelWriter struct {
	bytes.Buffer
	writes int
	limit  int
	cancel context.CancelFunc
}

func (w *cancelWriter) Write(data []byte) (int, error) {
	n, err := w.Buffer.Write(data)
	w.writes++
	if w.writes == w.limit {
		w.cancel()
	}
	return n, err
}

func TestHybridMatrixZoneCounts(t *testing.T) {
	d := testLight()
	d.Type = device.DeviceTypeHybrid
	d.LightType = device.LightTypeMatrix
	if zones(d) != "?" || viewDevice(d).ZoneCount != nil {
		t.Fatal("unknown hybrid geometry not marked")
	}
	d.MatrixProperties.Width = 8
	d.MatrixProperties.Height = 8
	d.MatrixProperties.ChainLength = 2
	if zones(d) != "128" || *viewDevice(d).ZoneCount != 128 {
		t.Fatal("hybrid matrix count missing")
	}
	d.Type = device.DeviceTypeSwitch
	if zones(d) != "-" || viewDevice(d).ZoneCount != nil {
		t.Fatal("switch given zones")
	}
}

func TestDevicesWatchLateGeometryAndRemoval(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := &cancelWriter{limit: 3, cancel: cancel}
	f := &fakeBackend{}
	reads := 0
	f.onGetDevices = func() []device.Device {
		reads++
		if reads == 3 {
			return nil
		}
		d := testLight()
		d.Type = device.DeviceTypeHybrid
		d.LightType = device.LightTypeMatrix
		if reads == 2 {
			d.MatrixProperties.Width = 8
			d.MatrixProperties.Height = 8
			d.MatrixProperties.ChainLength = 1
		}
		return []device.Device{d}
	}
	creates := 0
	err := run(ctx, []string{"devices", "--watch", "--discover-for", "1ns", "--interval", "1ns", "--output", "json"}, out, io.Discard, func(bool, io.Writer) (backend, error) { creates++; return f, nil })
	if !errors.Is(err, context.Canceled) || !f.closed || creates != 1 || f.sends != 0 || f.captures != 0 {
		t.Fatalf("err=%v creates=%d fake=%+v", err, creates, f)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatal(out.String())
	}
	for i, line := range lines {
		var frame inventoryView
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			t.Fatal(err)
		}
		if frame.Timestamp.IsZero() {
			t.Fatal("missing timestamp")
		}
		if i == 0 && (len(frame.Devices) != 1 || frame.Devices[0].ZoneCount != nil) {
			t.Fatal("first count should be unknown")
		}
		if i == 1 && (len(frame.Devices) != 1 || frame.Devices[0].ZoneCount == nil || *frame.Devices[0].ZoneCount != 64) {
			t.Fatal("late geometry not displayed")
		}
		if i == 2 && len(frame.Devices) != 0 {
			t.Fatal("removed device retained")
		}
	}
}

func TestDevicesWatchOutputFailureClosesController(t *testing.T) {
	f := &fakeBackend{}
	err := run(context.Background(), []string{"devices", "--watch", "--discover-for", "1ns"}, brokenWriter{}, io.Discard, factoryFor(f))
	if !errors.Is(err, io.ErrClosedPipe) || !f.closed {
		t.Fatalf("err=%v closed=%t", err, f.closed)
	}
}

func TestInventoryTerminalAndRedirectedFormatting(t *testing.T) {
	at := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	for _, inPlace := range []bool{true, false} {
		var out bytes.Buffer
		if err := printInventory(&out, []device.Device{testLight()}, "text", at, inPlace); err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(out.String(), "\x1b[H\x1b[2J") != inPlace {
			t.Fatal("unexpected terminal controls")
		}
		if !strings.Contains(out.String(), "2026-10-04T12:00:00Z") || !strings.Contains(out.String(), "SERIAL") {
			t.Fatal(out.String())
		}
	}
	if terminalOutput(&bytes.Buffer{}) {
		t.Fatal("buffer treated as terminal")
	}
}

func TestInvalidDevicesWatchIntervalIsOffline(t *testing.T) {
	for _, args := range [][]string{{"devices", "--watch", "--interval", "0s"}, {"devices", "--watch", "--interval", "-1s"}, {"devices", "--interval", "1s"}} {
		err := run(context.Background(), args, io.Discard, io.Discard, func(bool, io.Writer) (backend, error) { t.Fatal("opened controller"); return nil, nil })
		if err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestDevicesWatchSkipsDiscoveryWait(t *testing.T) {
	ctx, deadline := context.WithTimeout(context.Background(), time.Second)
	defer deadline()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	out := &cancelWriter{limit: 1, cancel: cancel}
	f := &fakeBackend{}
	err := run(ctx, []string{"devices", "--watch", "--discover-for", "1h", "--output", "json"}, out, io.Discard, factoryFor(f))
	if !errors.Is(err, context.Canceled) || out.writes != 1 || !f.closed {
		t.Fatalf("err=%v writes=%d closed=%t", err, out.writes, f.closed)
	}
	if !json.Valid(bytes.TrimSpace(out.Bytes())) {
		t.Fatal(out.String())
	}
}

func TestEmptyWatchInventoryIsStillDiscovering(t *testing.T) {
	var out bytes.Buffer
	if err := printInventory(&out, nil, "text", time.Now(), false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Waiting for devices") || strings.Contains(out.String(), "longer --discover-for") {
		t.Fatal(out.String())
	}
}
