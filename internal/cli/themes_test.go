package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

const themeJSON = `{"name":"Evening","palette":{"base":[{"hue":30,"saturation":80,"brightness":40,"kelvin":3500},{"hue":220,"saturation":80,"brightness":30,"kelvin":3500}]},"layout":"gradient"}`

func themeFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "theme.json")
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func themeLight() device.Device {
	d := testLight()
	d.ProductID = 27
	d.ColorProperties.HasColor = true
	return d
}

func TestThemeDryRunAndApplicationPreservePower(t *testing.T) {
	path := themeFile(t, themeJSON)
	for _, dry := range []bool{true, false} {
		d := themeLight()
		d.PoweredOn = false
		f := &fakeBackend{devices: []device.Device{d}}
		var out bytes.Buffer
		args := []string{"themes", "apply", "--target", "Desk", "--discover-for", "1ns"}
		if dry {
			args = append(args, "--dry-run")
		}
		args = append(args, path)
		if err := run(context.Background(), args, &out, io.Discard, factoryFor(f)); err != nil {
			t.Fatal(err)
		}
		if !f.closed || f.captures != 1 || f.restores != 0 || f.fresh {
			t.Fatalf("fake=%+v", f)
		}
		want := 1
		if dry {
			want = 0
		}
		if f.sends != want {
			t.Fatalf("sends=%d", f.sends)
		}
		for _, msg := range f.sentMessages {
			switch msg.Payload.(type) {
			case *packets.LightSetPower, *packets.DeviceSetPower:
				t.Fatal("theme changed power")
			}
		}
		if !strings.Contains(out.String(), "power preserved") || !strings.Contains(out.String(), "HSBK(") {
			t.Fatal(out.String())
		}
	}
}

func TestThemeMalformedInputAndSelectionAreOffline(t *testing.T) {
	for _, contents := range []string{`{"name":"bad","palette":{}}`, themeJSON + ` {}`, `{"unknown":1}`, `null`} {
		path := themeFile(t, contents)
		err := run(context.Background(), []string{"themes", "apply", "--target", "Desk", path}, io.Discard, io.Discard, func(bool, io.Writer) (backend, error) { t.Fatal("opened controller"); return nil, nil })
		if err == nil {
			t.Fatal("invalid theme accepted")
		}
	}
	path := themeFile(t, themeJSON)
	for _, flags := range [][]string{nil, {"--target", "Desk", "--filter", "group=Office"}, {"--target", "Desk", "--duration", "-1s"}} {
		args := append([]string{"themes", "apply"}, flags...)
		args = append(args, path)
		err := run(context.Background(), args, io.Discard, io.Discard, func(bool, io.Writer) (backend, error) { t.Fatal("opened controller"); return nil, nil })
		if err == nil {
			t.Fatal("invalid selection accepted")
		}
	}
}

func TestThemeCaptureOrOutputFailurePreventsSend(t *testing.T) {
	path := themeFile(t, themeJSON)
	for _, captureFails := range []bool{true, false} {
		f := &fakeBackend{devices: []device.Device{themeLight()}}
		if captureFails {
			f.captureErr = errors.New("missing matrix coverage")
		}
		err := run(context.Background(), []string{"themes", "apply", "--target", "Desk", "--discover-for", "1ns", path}, brokenWriter{}, io.Discard, factoryFor(f))
		if err == nil || f.sends != 0 || !f.closed {
			t.Fatalf("err=%v fake=%+v", err, f)
		}
	}
}

func TestThemeFilterSelectionAndPacketJSON(t *testing.T) {
	path := themeFile(t, themeJSON)
	d := themeLight()
	d.Group = "Office"
	other := d
	other.Serial[5]++
	other.Label = "Other"
	other.Group = "Home"
	f := &fakeBackend{devices: []device.Device{other, d}}
	var out bytes.Buffer
	err := run(context.Background(), []string{"themes", "apply", "--filter", "group=Office", "--dry-run", "--output", "json", "--discover-for", "1ns", path}, &out, io.Discard, factoryFor(f))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), d.Serial.String()) || strings.Contains(out.String(), other.Serial.String()) || strings.Contains(out.String(), "Theme ") || f.sends != 0 {
		t.Fatal(out.String())
	}
}

func TestThemeSwitchSelectionRejectedBeforeCapture(t *testing.T) {
	path := themeFile(t, themeJSON)
	d := themeLight()
	d.Type = device.DeviceTypeSwitch
	f := &fakeBackend{devices: []device.Device{d}}
	err := run(context.Background(), []string{"themes", "apply", "--target", "Desk", "--discover-for", "1ns", path}, io.Discard, io.Discard, factoryFor(f))
	if err == nil || f.captures != 0 || f.sends != 0 {
		t.Fatalf("err=%v fake=%+v", err, f)
	}
}

func TestThemeHybridMatrixPacketsAndSendFailure(t *testing.T) {
	path := themeFile(t, themeJSON)
	d := themeLight()
	d.Type = device.DeviceTypeHybrid
	d.LightType = device.LightTypeMatrix
	d.MatrixProperties = device.MatrixProperties{Width: 8, Height: 8, NZones: 64, ChainLength: 1, ChainZones: [][]packets.LightHsbk{make([]packets.LightHsbk, 64)}}
	f := &fakeBackend{devices: []device.Device{d}, sendErr: errors.New("offline")}
	err := run(context.Background(), []string{"themes", "apply", "--target", "Desk", "--discover-for", "1ns", path}, io.Discard, io.Discard, factoryFor(f))
	if err == nil || !strings.Contains(err.Error(), "earlier sends may already have applied") || f.sends != 1 || f.restores != 0 {
		t.Fatalf("err=%v fake=%+v", err, f)
	}
	if _, ok := f.sentMessages[0].Payload.(*packets.TileSet64); !ok {
		t.Fatalf("unexpected payload %T", f.sentMessages[0].Payload)
	}
}

func TestThemeExcessiveGeometryPreventsControlWrites(t *testing.T) {
	d := themeLight()
	d.LightType = device.LightTypeMatrix
	d.MatrixProperties = device.MatrixProperties{Width: math.MaxInt, Height: math.MaxInt, ChainLength: math.MaxInt}
	f := &fakeBackend{devices: []device.Device{d}}
	err := run(context.Background(), []string{"themes", "apply", "--target", "Desk", "--discover-for", "1ns", themeFile(t, themeJSON)}, io.Discard, io.Discard, factoryFor(f))
	if err == nil || f.sends != 0 || f.restores != 0 {
		t.Fatalf("err=%v fake=%+v", err, f)
	}
}
