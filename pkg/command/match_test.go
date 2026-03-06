package command

import (
	"testing"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/stretchr/testify/assert"
)

func TestMatch(t *testing.T) {
	var (
		serial0 = device.Serial([8]byte{0, 0, 0, 0, 0, 0})
		serial1 = device.Serial([8]byte{0, 0, 0, 0, 0, 1})
		serial2 = device.Serial([8]byte{0, 0, 0, 0, 0, 2})
		serial3 = device.Serial([8]byte{0, 0, 0, 0, 0, 3})

		devices = []device.Device{
			{Serial: serial0, Label: "mOOn", Group: "tv", Location: "home"},
			{Serial: serial1, Label: "luna", Group: "Living Room", Location: "home"},
			{Serial: serial2, Label: "Neon", Group: "Living Room", Location: "home"},
			{Serial: serial3, Label: "filo", Group: "tv", Location: "home"},
		}
		cmdParser = NewCommandParser(devices)
	)

	testCases := map[string]struct {
		term string
		want []string
	}{
		"exact":    {term: "moon", want: []string{"mOOn"}},
		"prefix":   {term: "mo", want: []string{"mOOn"}},
		"contains": {term: "on", want: []string{"mOOn", "Neon"}},
		"fuzzy":    {term: "no", want: []string{"Neon", "Living Room"}},
		"no match": {term: "xyz", want: []string{}},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := cmdParser.Match(tc.term)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestMatchScore(t *testing.T) {
	testCases := map[string]struct {
		key  string
		term string
		want int
	}{
		"exact":    {key: "kitchen", term: "kitchen", want: 100},
		"prefix":   {key: "kitchen light", term: "kit", want: 75},
		"contains": {key: "main kitchen", term: "kit", want: 50},
		"fuzzy":    {key: "kitchen", term: "kchn", want: 15},
		"no match": {key: "kitchen", term: "xyz", want: 0},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := matchScore(tc.key, tc.term)
			assert.Equal(t, tc.want, got)
		})
	}
}

func Test_fuzzyScore(t *testing.T) {
	testCases := map[string]struct {
		key  string
		term string
		want int
	}{
		// ---- Exact / Full match ----
		"exact match full score": {key: "kitchen", term: "kitchen", want: 25},
		"exact short word":       {key: "moon", term: "moon", want: 24},

		// ---- Proportional length scoring ----
		"short word higher density": {key: "moon", term: "moo", want: 23},
		"long word lower density":   {key: "marvin room", term: "moo", want: 10},

		// ---- Subsequence ----
		"subsequence mid word":    {key: "kitchen", term: "kchn", want: 15},
		"short start subsequence": {key: "bedroom", term: "bd", want: 17},

		// ---- Prefix preference (same density) ----
		"prefix match":              {key: "neon", term: "ne", want: 22},
		"middle match same density": {key: "zone", term: "ne", want: 19},

		// ---- Order sensitivity ----
		"order matters": {key: "kitchen", term: "nhk", want: 0},

		// ---- Edge cases ----
		"term longer than key": {key: "bed", term: "bedroom", want: 0},
		"not found":            {key: "kitchen", term: "xyz", want: 0},
		"empty term":           {key: "kitchen", term: "", want: 0},
		"empty key":            {key: "", term: "abc", want: 0},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := fuzzyScore(tc.key, tc.term)
			assert.Equal(t, tc.want, got)
		})
	}
}

// realisticDevices returns a slice of simulated devices for benchmarking.
func realisticDevices() []device.Device {
	return []device.Device{
		{Label: "Kitchen Light", Group: "Main", Serial: device.Serial([8]byte{0, 0, 0, 0, 0, 0})},
		{Label: "Living Room Light", Group: "Main", Serial: device.Serial([8]byte{0, 0, 0, 0, 0, 1})},
		{Label: "Bedroom Lamp", Group: "Upstairs", Serial: device.Serial([8]byte{0, 0, 0, 0, 0, 2})},
		{Label: "Neon Sign", Group: "Office", Serial: device.Serial([8]byte{0, 0, 0, 0, 0, 3})},
		{Label: "Marvin Room Lamp", Group: "Office", Serial: device.Serial([8]byte{0, 0, 0, 0, 0, 4})},
		{Label: "Garage Light", Group: "Outside", Serial: device.Serial([8]byte{0, 0, 0, 0, 0, 5})},
		{Label: "Bathroom Ceiling", Group: "Upstairs", Serial: device.Serial([8]byte{0, 0, 0, 0, 0, 6})},
		{Label: "Moon Lamp", Group: "Kids Room", Serial: device.Serial([8]byte{0, 0, 0, 0, 0, 7})},
		{Label: "Main Kitchen Area", Group: "Main", Serial: device.Serial([8]byte{0, 0, 0, 0, 0, 8})},
		{Label: "Bedside Light", Group: "Upstairs", Serial: device.Serial([8]byte{0, 0, 0, 0, 0, 9})},
	}
}

func BenchmarkMatchScore(b *testing.B) {
	keys := []string{
		"Kitchen Light", "Living Room Light", "Bedroom Lamp", "Neon Sign",
		"Marvin Room Lamp", "Garage Light", "Bathroom Ceiling", "Moon Lamp",
		"Main Kitchen Area", "Bedside Light",
	}

	terms := []string{"ki", "bed", "no", "moo", "kit", "xyz", "moon"}

	b.ResetTimer()
	for b.Loop() {
		for _, key := range keys {
			for _, term := range terms {
				_ = matchScore(key, term)
			}
		}
	}
}

func BenchmarkCommandParserMatch(b *testing.B) {
	devices := realisticDevices()
	parser := NewCommandParser(devices)

	terms := []string{"ki", "bed", "no", "moon", "neon", "garage", "lamp", "main"}

	b.ResetTimer()
	for b.Loop() {
		for _, term := range terms {
			_ = parser.Match(term)
		}
	}
}
