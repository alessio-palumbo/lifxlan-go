package device

import (
	"reflect"
	"testing"
)

func TestSplitSelectors(t *testing.T) {
	got := SplitSelectors(" tv, desk ,, living room ")
	want := []string{"tv", "desk", "living room"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selectors = %#v, want %#v", got, want)
	}
}

func TestMatchSelectorMatchesAll(t *testing.T) {
	devices := selectorDevices(t)

	got := MatchSelector("ALL", devices)

	if !reflect.DeepEqual(serialsFromDevices(got), []Serial{devices[0].Serial, devices[1].Serial, devices[2].Serial}) {
		t.Fatalf("serials = %#v", serialsFromDevices(got))
	}
	got[0].Label = "mutated"
	if devices[0].Label == "mutated" {
		t.Fatal("MatchSelector returned a mutable backing device")
	}
}

func TestMatchSelectorMatchesSerialLabelGroupAndLocation(t *testing.T) {
	devices := selectorDevices(t)

	tests := map[string][]Serial{
		devices[0].Serial.String(): {devices[0].Serial},
		"tv":                       {devices[0].Serial},
		"OFFICE":                   {devices[1].Serial},
		"home":                     {devices[0].Serial, devices[1].Serial},
	}

	for selector, want := range tests {
		t.Run(selector, func(t *testing.T) {
			got := serialsFromDevices(MatchSelector(selector, devices))
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("serials = %#v, want %#v", got, want)
			}
		})
	}
}

func TestMatchSelectorDeDuplicatesDeviceMatchedByMultipleFields(t *testing.T) {
	serial := mustSerial(t, "001122334455")
	devices := []Device{{Serial: serial, Label: "Desk", Group: "Desk", Location: "Office"}}

	got := MatchSelector("desk", devices)

	if !reflect.DeepEqual(serialsFromDevices(got), []Serial{serial}) {
		t.Fatalf("serials = %#v, want %s", serialsFromDevices(got), serial)
	}
}

func TestResolveSelectorsPreservesSelectorOrderAndDeDuplicates(t *testing.T) {
	devices := selectorDevices(t)

	got := ResolveSelectorSerials("office, all, tv", devices)
	want := []Serial{devices[1].Serial, devices[0].Serial, devices[2].Serial}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("serials = %#v, want %#v", got, want)
	}
}

func TestResolveSelectorsHandlesEmptyAndUnknownSelectors(t *testing.T) {
	devices := selectorDevices(t)

	if got := ResolveSelectors("", devices); got != nil {
		t.Fatalf("empty selector = %#v, want nil", got)
	}
	if got := ResolveSelectorSerials("missing", devices); got != nil {
		t.Fatalf("missing selector = %#v, want nil", got)
	}
}

func selectorDevices(t *testing.T) []Device {
	t.Helper()
	return []Device{
		{Serial: mustSerial(t, "001122334455"), Label: "TV", Group: "Lounge", Location: "Home"},
		{Serial: mustSerial(t, "aabbccddeeff"), Label: "Desk", Group: "Office", Location: "Home"},
		{Serial: mustSerial(t, "112233445566"), Label: "Neon", Group: "Lounge", Location: "Studio"},
	}
}

func mustSerial(t *testing.T, value string) Serial {
	t.Helper()
	serial, err := SerialFromHex(value)
	if err != nil {
		t.Fatal(err)
	}
	return serial
}

func serialsFromDevices(devices []Device) []Serial {
	serials := make([]Serial, len(devices))
	for i, d := range devices {
		serials[i] = d.Serial
	}
	return serials
}
