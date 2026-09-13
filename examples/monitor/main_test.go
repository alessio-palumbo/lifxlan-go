package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alessio-palumbo/lifxlan-go/pkg/controller"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

func TestPrintDevicesRepaintsInteractiveTerminal(t *testing.T) {
	serial := device.Serial{1}
	devices := map[device.Serial]device.Device{
		serial: {
			Serial:       serial,
			Label:        "Desk",
			Location:     "Home",
			Group:        "Office",
			ProductID:    27,
			RegistryName: "LIFX A19",
			Type:         device.DeviceTypeLight,
			PoweredOn:    true,
			Color:        device.Color{Hue: 210, Saturation: 80, Brightness: 60, Kelvin: 3500},
		},
	}

	var output bytes.Buffer
	printDevices(&output, controller.DeviceEvent{
		Type:     controller.DeviceEventUpdated,
		Revision: 7,
	}, devices, true)

	got := output.String()
	if !strings.HasPrefix(got, "\x1b[H\x1b[2J") {
		t.Fatalf("output does not repaint terminal: %q", got)
	}
	for _, want := range []string{"event=updated", "revision=7", "Desk", "Home", "Office", "LIFX A19", "h=210.0 s=80.0 b=60.0 k=3500"} {
		if !strings.Contains(got, want) {
			t.Errorf("output does not contain %q:\n%s", want, got)
		}
	}
}

func TestPrintDevicesUsesPlainOutputWhenRedirected(t *testing.T) {
	var output bytes.Buffer
	printDevices(&output, controller.DeviceEvent{Type: controller.DeviceEventAdded}, nil, false)
	if strings.Contains(output.String(), "\x1b[") {
		t.Fatalf("plain output contains terminal control sequence: %q", output.String())
	}
}

func TestPrintDevicesNamesSnapshotComplete(t *testing.T) {
	var output bytes.Buffer
	printDevices(&output, controller.DeviceEvent{
		Type:     controller.DeviceEventSnapshotComplete,
		Revision: 12,
	}, nil, false)
	if !strings.Contains(output.String(), "event=snapshot_complete  revision=12") {
		t.Fatalf("snapshot-complete output = %q", output.String())
	}
}
