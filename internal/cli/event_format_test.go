package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/controller"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func TestStreamLightUpdateShowsColorAndTime(t *testing.T) {
	d := testLight()
	d.ProductID = 27
	d.Color = device.Color{Hue: 220, Saturation: 80, Brightness: 35, Kelvin: 3500}
	var out bytes.Buffer
	err := printEventAt(&out, controller.DeviceEvent{Type: controller.DeviceEventUpdated, Revision: 8, Device: d, Changes: controller.DeviceChangeLight}, time.Date(2026, 10, 4, 12, 0, 0, 123000000, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"2026-10-04T12:00:00.123Z", "updated", "changes=light", "power=on", "color=HSBK(220.0,80.0,35.0,3500)"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %s in %s", want, out.String())
		}
	}
}

func TestStreamInitialIsExplicitAndMetadataChangesAreRelevant(t *testing.T) {
	d := testLight()
	d.FirmwareVersion = "3.90"
	d.Group = "Office"
	d.Label = "Desk\x1b[2J\n"
	var out bytes.Buffer
	if err := printEvent(&out, controller.DeviceEvent{Type: controller.DeviceEventAdded, Initial: true, Device: d}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "initial") || !strings.Contains(out.String(), "cached:") || strings.Contains(out.String(), "\x1b") {
		t.Fatal(out.String())
	}
	out.Reset()
	if err := printEvent(&out, controller.DeviceEvent{Type: controller.DeviceEventUpdated, Device: d, Changes: controller.DeviceChangeFirmware | controller.DeviceChangeGroup}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "changes=firmware,group") || !strings.Contains(out.String(), "firmware=3.90") || !strings.Contains(out.String(), `group="Office"`) || strings.Contains(out.String(), "power=") {
		t.Fatal(out.String())
	}
}

func TestStreamMatrixPreviewIsBoundedAndMarkedCached(t *testing.T) {
	d := testLight()
	d.Type = device.DeviceTypeHybrid
	d.LightType = device.LightTypeMatrix
	d.MatrixProperties.Width = 8
	d.MatrixProperties.Height = 8
	d.MatrixProperties.ChainLength = 1
	d.MatrixProperties.ChainZones = [][]packets.LightHsbk{make([]packets.LightHsbk, 64)}
	d.MatrixProperties.ChainZones[0][63].Brightness = 65535
	var out bytes.Buffer
	if err := printEvent(&out, controller.DeviceEvent{Type: controller.DeviceEventUpdated, Device: d, Changes: controller.DeviceChangeMatrix}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"changes=matrix", "zones=64", "geometry=8x8", "cached_colors=64", "brightness=0.0..100.0%", "... +61"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %s in %s", want, out.String())
		}
	}
	if strings.Count(out.String(), "HSBK(") != 3 {
		t.Fatal("unbounded zone preview")
	}
}

func TestStreamEveryChangeCategoryIsNamed(t *testing.T) {
	d := testLight()
	d.Group = "Group"
	d.Location = "Location"
	for _, category := range changeCategories {
		var out bytes.Buffer
		if err := printEvent(&out, controller.DeviceEvent{Type: controller.DeviceEventUpdated, Device: d, Changes: category.mask}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "changes="+category.name) {
			t.Fatal(out.String())
		}
	}
	var out bytes.Buffer
	if err := printEvent(&out, controller.DeviceEvent{Type: controller.DeviceEventUpdated, Device: d, Changes: controller.DeviceChange(1 << 63)}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "unknown(0x8000000000000000)") {
		t.Fatal(out.String())
	}
}
