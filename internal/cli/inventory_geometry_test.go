package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

func TestInventoryVisibleMatrixZones(t *testing.T) {
	for _, test := range []struct {
		pid                                    uint32
		width, height, pixels, chains, visible int
	}{
		{215, 5, 6, 30, 1, 27},
		{215, 5, 6, 30, 2, 54},
		{57, 5, 11, 55, 1, 52},
		{145, 8, 8, 64, 1, 56},
		{201, 8, 16, 128, 1, 120},
		{219, 7, 5, 35, 1, 31},
		{55, 8, 8, 64, 5, 320},
	} {
		d := testLight()
		d.ProductID = test.pid
		d.LightType = device.LightTypeMatrix
		d.MatrixProperties = device.MatrixProperties{Width: test.width, Height: test.height, NZones: test.pixels, ChainLength: test.chains}
		before := d.MatrixProperties
		got := viewDevice(d).ZoneCount
		if got == nil || *got != test.visible {
			t.Fatalf("pid=%d chains=%d count=%v want=%d", test.pid, test.chains, got, test.visible)
		}
		if d.MatrixProperties.Width != before.Width || d.MatrixProperties.NZones != before.NZones {
			t.Fatal("display changed physical geometry")
		}
	}
}

func TestInventoryProductNameFormattingAndJSON(t *testing.T) {
	d := testLight()
	for _, test := range []struct{ name, want string }{
		{"", "-"}, {"LIFX Candle Colour", "Candle Colour"},
		{"LIFX Luna", "Luna"}, {"Other Product", "Other Product"},
		{"LIFX Bad\nName\t", "Bad Name"},
	} {
		d.RegistryName = test.name
		if got := productName(d); got != test.want {
			t.Fatalf("got=%q want=%q", got, test.want)
		}
	}
	d.RegistryName = "LIFX Candle Colour"
	var out bytes.Buffer
	if err := printDevices(&out, []device.Device{d}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Candle Colour") || strings.Contains(out.String(), "LIFX Candle") {
		t.Fatal(out.String())
	}
	out.Reset()
	if err := writeJSON(&out, viewDevice(d)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"RegistryName": "LIFX Candle Colour"`) {
		t.Fatal(out.String())
	}
}
