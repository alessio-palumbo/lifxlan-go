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

	"github.com/alessio-palumbo/lifxlan-go/pkg/controller"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

func TestListFilterANDAndOR(t *testing.T) {
	filters, err := parseFilters([]string{"location=Office", "group=Desk", "group=Ceiling", "type=MATRIX", "product_id=027", "firmware_version=3.90"})
	if err != nil {
		t.Fatal(err)
	}
	d := testLight()
	d.ProductID = 27
	d.Location = "office"
	d.Group = "ceiling"
	d.LightType = device.LightTypeMatrix
	d.FirmwareVersion = "3.90"
	if !filters.matches(d) {
		t.Fatal("matching device rejected")
	}
	d.Group = "Other"
	if filters.matches(d) {
		t.Fatal("group OR accepted unrelated value")
	}
	d.Group = "Desk"
	d.FirmwareVersion = ""
	if filters.matches(d) {
		t.Fatal("missing metadata accepted")
	}
	d.FirmwareVersion = "3.90"
	d.Location = "Home"
	if filters.matches(d) {
		t.Fatal("different keys not ANDed")
	}
}

func TestInvalidFiltersAreOffline(t *testing.T) {
	for _, filter := range []string{"bad=value", "location=", "type=hybrid", "product_id=-1", "product_id=0", "product_id=4294967296", "firmware_version=invalid", "location", "serial=xyz", "serial=d073d500000z", "serial=d073d500000100", "label="} {
		err := run(context.Background(), []string{"devices", "list", "--filter", filter}, io.Discard, io.Discard, func(bool, io.Writer) (backend, error) { t.Fatal("opened controller"); return nil, nil })
		if err == nil {
			t.Fatalf("accepted %q", filter)
		}
	}
}

func TestSerialAndLabelFilters(t *testing.T) {
	d := testLight()
	d.Location = "Office"
	other := d
	other.Serial[5]++
	other.Label = "Other"
	duplicate := d
	duplicate.Serial[5] += 2
	filters, err := parseFilters([]string{"serial=" + strings.ToUpper(d.Serial.String()), "serial=" + other.Serial.String(), "label=desk", "label=OTHER", "location=office"})
	if err != nil {
		t.Fatal(err)
	}
	if got := filters.apply([]device.Device{d, other, duplicate}); len(got) != 2 {
		t.Fatalf("selected %d devices", len(got))
	}
	labels, err := parseFilters([]string{"label=desk"})
	if err != nil {
		t.Fatal(err)
	}
	if got := labels.apply([]device.Device{d, other, duplicate}); len(got) != 2 {
		t.Fatal("duplicate label rejected")
	}
	d.Label = "Desktop"
	if labels.matches(d) {
		t.Fatal("label matched a substring")
	}
	d.Label = ""
	if labels.matches(d) {
		t.Fatal("missing label matched")
	}
}

func TestWatchedLabelFilterReevaluatesArrivalAndRename(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := &cancelWriter{limit: 3, cancel: cancel}
	f := &fakeBackend{}
	reads := 0
	f.onGetDevices = func() []device.Device {
		reads++
		d := testLight()
		switch reads {
		case 1:
			d.Label = ""
		case 3:
			d.Label = "Renamed"
		}
		return []device.Device{d}
	}
	err := run(ctx, []string{"devices", "list", "--watch", "--interval", "1ns", "--output", "json", "--filter", "label=Desk"}, out, io.Discard, factoryFor(f))
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
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
		want := 0
		if i == 1 {
			want = 1
		}
		if len(frame.Devices) != want {
			t.Fatalf("frame %d: %s", i, line)
		}
	}
}

func TestListFilterRouteAndLiteralComma(t *testing.T) {
	d := testLight()
	d.ProductID = 27
	d.Location = "Office, upstairs"
	d.LightType = device.LightTypeMatrix
	other := d
	other.Label = "Other"
	other.Location = "Home"
	f := &fakeBackend{devices: []device.Device{d, other}}
	var out bytes.Buffer
	err := run(context.Background(), []string{"devices", "list", "--discover-for", "1ns", "--output", "json", "--filter", "location=Office, upstairs", "--filter", "type=matrix"}, &out, io.Discard, factoryFor(f))
	if err != nil {
		t.Fatal(err)
	}
	var views []deviceView
	if err := json.Unmarshal(out.Bytes(), &views); err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 || views[0].Label != "Desk" {
		t.Fatal(out.String())
	}
}

func TestWatchedFilterReevaluatesLateMetadata(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := &cancelWriter{limit: 2, cancel: cancel}
	f := &fakeBackend{}
	reads := 0
	f.onGetDevices = func() []device.Device {
		reads++
		d := testLight()
		if reads > 1 {
			d.Location = "Office"
		}
		return []device.Device{d}
	}
	err := run(ctx, []string{"devices", "list", "--watch", "--interval", "1ns", "--output", "json", "--filter", "location=Office"}, out, io.Discard, factoryFor(f))
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatal(out.String())
	}
	for i, line := range lines {
		var frame inventoryView
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			t.Fatal(err)
		}
		if len(frame.Devices) != i {
			t.Fatalf("frame %d: %s", i, line)
		}
	}
}

func TestStreamSerialImmediateAndControlEventsPreserved(t *testing.T) {
	d := testLight()
	other := d
	other.Serial[5]++
	f := &fakeBackend{events: []controller.DeviceEvent{
		{Type: controller.DeviceEventAdded, Device: other},
		{Type: controller.DeviceEventAdded, Device: d},
		{Type: controller.DeviceEventSnapshotComplete},
		{Type: controller.DeviceEventResyncRequired},
	}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var out bytes.Buffer
	err := run(ctx, []string{"devices", "stream", "--target", d.Serial.String(), "--discover-for", "1h", "--output", "json"}, &out, io.Discard, factoryFor(f))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 || f.subscriptions != 1 || !f.closed {
		t.Fatal(out.String())
	}
	var event controller.DeviceEvent
	if err := json.Unmarshal([]byte(lines[0]), &event); err != nil {
		t.Fatal(err)
	}
	if event.Device.Serial != d.Serial {
		t.Fatal("unrelated event emitted")
	}
}

func TestStreamLabelsAndAmbiguity(t *testing.T) {
	d := testLight()
	for _, duplicate := range []bool{false, true} {
		f := &fakeBackend{devices: []device.Device{d}, events: []controller.DeviceEvent{{Type: controller.DeviceEventAdded, Device: d}}}
		if duplicate {
			other := d
			other.Serial[5]++
			f.devices = append(f.devices, other)
		}
		var out bytes.Buffer
		err := run(context.Background(), []string{"devices", "stream", "--target", "desk", "--discover-for", "1ns"}, &out, io.Discard, factoryFor(f))
		if err != nil || !strings.Contains(out.String(), "Desk") || f.subscriptions != 1 {
			t.Fatalf("err=%v output=%s", err, out.String())
		}
	}
}

func TestDeviceGroupHelpAndListFlagsNotAcceptedByStream(t *testing.T) {
	var out bytes.Buffer
	err := run(context.Background(), []string{"devices", "--help"}, &out, io.Discard, func(bool, io.Writer) (backend, error) { t.Fatal("opened controller"); return nil, nil })
	if err != nil || !strings.Contains(out.String(), "stream") || !strings.Contains(out.String(), "inspect") || !strings.Contains(out.String(), "list") {
		t.Fatalf("err=%v help=%s", err, out.String())
	}
	err = run(context.Background(), []string{"devices", "stream", "--filter", "location=Office"}, io.Discard, io.Discard, func(bool, io.Writer) (backend, error) { t.Fatal("opened controller"); return nil, nil })
	if err == nil {
		t.Fatal("stream accepted list filters")
	}
}
