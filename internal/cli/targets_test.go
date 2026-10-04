package cli

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/alessio-palumbo/lifxlan-go/pkg/controller"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

func selectorDevices() []device.Device {
	a := themeLight()
	a.Group = "Office"
	a.Location = "Home"
	b := a
	b.Label = "Beam"
	b.Serial[5]++
	switchDevice := a
	switchDevice.Label = "Switch"
	switchDevice.Serial[5] += 2
	switchDevice.Type = device.DeviceTypeSwitch
	return []device.Device{a, b, switchDevice}
}

func TestSelectorsReuseGrammarAndDeduplicate(t *testing.T) {
	devices := selectorDevices()
	selected, err := resolveTargets([]string{"Desk,group:Office", "location:Home,serial:" + devices[0].Serial.String()}, devices, true)
	if err != nil || len(selected) != 2 {
		t.Fatalf("err=%v count=%d", err, len(selected))
	}
	selected, err = resolveTargets([]string{"ALL"}, devices, false)
	if err != nil || len(selected) != 3 {
		t.Fatalf("err=%v count=%d", err, len(selected))
	}
	for _, input := range []string{"Desk,missing", "bad:Office", "serial:bad", "Switch", " , "} {
		if _, err := resolveTargets([]string{input}, devices, true); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
}

func TestBareNamesUnionAndTypedDisambiguation(t *testing.T) {
	devices := selectorDevices()
	devices[0].Label = "Office"
	if _, err := selectDevice(devices, "Office"); err == nil {
		t.Fatal("single-device command accepted a name collision")
	}
	got, err := selectDevice(devices, "label:Office")
	if err != nil || got.Serial != devices[0].Serial {
		t.Fatalf("got=%v err=%v", got.Serial, err)
	}
}

func TestThemeSelectorGroupsListsAllAndDuplicateLabels(t *testing.T) {
	path := themeFile(t, themeJSON)
	for _, selector := range []string{"Office", "group:Office", "location:Home", "Desk,Beam", "all"} {
		f := &fakeBackend{devices: selectorDevices()}
		var out bytes.Buffer
		err := run(context.Background(), []string{"themes", "apply", "--target", selector, "--dry-run", "--discover-for", "1ns", path}, &out, io.Discard, factoryFor(f))
		if err != nil || len(f.capturedSerials) != 2 || f.sends != 0 {
			t.Fatalf("%s: err=%v serials=%v", selector, err, f.capturedSerials)
		}
	}
	f := &fakeBackend{devices: selectorDevices()}
	f.devices[1].Label = "Desk"
	if err := run(context.Background(), []string{"themes", "apply", "--target", "label:Desk", "--dry-run", "--discover-for", "1ns", path}, io.Discard, io.Discard, factoryFor(f)); err != nil || len(f.capturedSerials) != 2 {
		t.Fatalf("err=%v serials=%v", err, f.capturedSerials)
	}
}

func TestUnmatchedThemeTokenAbortsBeforeCaptureAndSend(t *testing.T) {
	f := &fakeBackend{devices: selectorDevices()}
	path := themeFile(t, themeJSON)
	err := run(context.Background(), []string{"themes", "apply", "--target", "Desk,missing", "--discover-for", "1ns", path}, io.Discard, io.Discard, factoryFor(f))
	if err == nil || f.captures != 0 || f.sends != 0 {
		t.Fatalf("err=%v fake=%+v", err, f)
	}
}

func TestSnapshotSelectorsIncludeOnlyLights(t *testing.T) {
	f := &fakeBackend{devices: selectorDevices()}
	err := run(context.Background(), []string{"snapshot", "--target", "all", "--discover-for", "1ns"}, io.Discard, io.Discard, factoryFor(f))
	if err != nil || len(f.capturedSerials) != 2 {
		t.Fatalf("err=%v serials=%v", err, f.capturedSerials)
	}
}

func TestStreamGroupAndCommaSeparatedSerials(t *testing.T) {
	devices := selectorDevices()
	for _, target := range []string{"group:Office", devices[0].Serial.String() + ",serial:" + devices[1].Serial.String()} {
		f := &fakeBackend{devices: devices, events: []controller.DeviceEvent{{Type: controller.DeviceEventAdded, Device: devices[0]}, {Type: controller.DeviceEventAdded, Device: devices[1]}, {Type: controller.DeviceEventAdded, Device: devices[2]}}}
		var out bytes.Buffer
		if err := run(context.Background(), []string{"devices", "stream", "--target", target, "--discover-for", "1ns"}, &out, io.Discard, factoryFor(f)); err != nil {
			t.Fatal(err)
		}
		want := 2
		if target == "group:Office" {
			want = 3
		}
		if lines := strings.Split(strings.TrimSpace(out.String()), "\n"); len(lines) != want {
			t.Fatal(out.String())
		}
	}
}
