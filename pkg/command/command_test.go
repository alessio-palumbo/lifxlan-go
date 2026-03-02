package command

import (
	"testing"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	var (
		serial0 = device.Serial([8]byte{0, 0, 0, 0, 0, 0})
		serial1 = device.Serial([8]byte{0, 0, 0, 0, 0, 1})
		serial2 = device.Serial([8]byte{0, 0, 0, 0, 0, 2})
		serial3 = device.Serial([8]byte{0, 0, 0, 0, 0, 3})

		devices = []device.Device{
			{Serial: serial0, Label: "moon", Group: "tv", Location: "home"},
			{Serial: serial1, Label: "luna", Group: "living room", Location: "home"},
			{Serial: serial2, Label: "neon", Group: "living room", Location: "home"},
			{Serial: serial3, Label: "filo", Group: "tv", Location: "home"},
		}
	)

	testCases := map[string]struct {
		input string
		want  []Command
	}{
		"serial": {
			input: "set 000000000000 to blue",
			want: []Command{
				{
					Targets: []device.Serial{serial0},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1000, SetHue: true, SetSaturation: true,
							Color: packets.LightHsbk{Hue: 45510, Saturation: 65535},
						}),
					},
				},
			},
		},
		"label": {
			input: "set moon to blue",
			want: []Command{
				{
					Targets: []device.Serial{serial0},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1000, SetHue: true, SetSaturation: true,
							Color: packets.LightHsbk{Hue: 45510, Saturation: 65535},
						}),
					},
				},
			},
		},
		"multi target": {
			input: "set moon and living room to green",
			want: []Command{
				{
					Targets: []device.Serial{serial0, serial1, serial2},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1000, SetHue: true, SetSaturation: true,
							Color: packets.LightHsbk{Hue: 21845, Saturation: 65535},
						}),
					},
				},
			},
		},
		"just keywords": {
			input: "home green",
			want: []Command{
				{
					Targets: []device.Serial{serial0, serial1, serial2, serial3},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1000, SetHue: true, SetSaturation: true,
							Color: packets.LightHsbk{Hue: 21845, Saturation: 65535},
						}),
					},
				},
			},
		},
		"just keywords: flipped": {
			input: "off home",
			want: []Command{
				{
					Targets: []device.Serial{serial0, serial1, serial2, serial3},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.DeviceSetPower{Level: 0}),
					},
				},
			},
		},
		"single action multiple targets: action last": {
			input: "set moon and luna blue",
			want: []Command{
				{
					Targets: []device.Serial{serial0, serial1},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1000, SetHue: true, SetSaturation: true,
							Color: packets.LightHsbk{Hue: 45510, Saturation: 65535},
						}),
					},
				},
			},
		},
		"single action multiple targets: action first, consecutive targets": {
			input: "set to blue moon luna",
			want: []Command{
				{
					Targets: []device.Serial{serial0, serial1},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1000, SetHue: true, SetSaturation: true,
							Color: packets.LightHsbk{Hue: 45510, Saturation: 65535},
						}),
					},
				},
			},
		},
		"single action multiple targets: action first, non consecutive targets": {
			input: "set to blue moon and luna",
			want: []Command{
				{
					Targets: []device.Serial{serial0, serial1},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1000, SetHue: true, SetSaturation: true,
							Color: packets.LightHsbk{Hue: 45510, Saturation: 65535},
						}),
					},
				},
			},
		},
		"single target, multiple actions": {
			input: "set luna to green, brightness 30%",
			want: []Command{
				{
					Targets: []device.Serial{serial1},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1000, SetHue: true, SetSaturation: true, SetBrightness: true,
							Color: packets.LightHsbk{Hue: 21845, Saturation: 65535, Brightness: 19661},
						}),
					},
				},
			},
		},
		"multiple targets, multiple actions: targets first": {
			input: "set luna and moon to green and brightness 30%",
			want: []Command{
				{
					Targets: []device.Serial{serial1, serial0},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1000, SetHue: true, SetSaturation: true, SetBrightness: true,
							Color: packets.LightHsbk{Hue: 21845, Saturation: 65535, Brightness: 19661},
						}),
					},
				},
			},
		},
		"multiple targets, multiple actions: actions first": {
			input: "set to green and brightness 30% luna and moon",
			want: []Command{
				{
					Targets: []device.Serial{serial1, serial0},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1000, SetHue: true, SetSaturation: true, SetBrightness: true,
							Color: packets.LightHsbk{Hue: 21845, Saturation: 65535, Brightness: 19661},
						}),
					},
				},
			},
		},
		"multiple targets, different actions": {
			input: "set luna to green, moon to brightness 30%",
			want: []Command{
				{
					Targets: []device.Serial{serial1},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1000, SetHue: true, SetSaturation: true,
							Color: packets.LightHsbk{Hue: 21845, Saturation: 65535},
						}),
					},
				},
				{
					Targets: []device.Serial{serial0},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1000, SetBrightness: true,
							Color: packets.LightHsbk{Brightness: 19661},
						}),
					},
				},
			},
		},
		"multiple property words with terminating token": {
			input: "set 000000000000 to 10% sat, 180 hue, 4000k and switch off. turn on luna to 10% brightness and 5000 kelvin",
			want: []Command{
				{
					Targets: []device.Serial{serial0},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1000, SetHue: true, SetSaturation: true, SetKelvin: true,
							Color: packets.LightHsbk{Hue: 32768, Saturation: 6554, Kelvin: 4000},
						}),
						protocol.NewMessage(&packets.DeviceSetPower{Level: 0}),
					},
				},
				{
					Targets: []device.Serial{serial1},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1000, SetBrightness: true, SetKelvin: true,
							Color: packets.LightHsbk{Brightness: 6554, Kelvin: 5000},
						}),
						protocol.NewMessage(&packets.DeviceSetPower{Level: 65535}),
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			cmdParser := NewCommandParser(devices)
			got := cmdParser.Parse(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}

func Test_matchEntities(t *testing.T) {
	var (
		serial0 = device.Serial([8]byte{0, 0, 0, 0, 0, 0})
		serial1 = device.Serial([8]byte{0, 0, 0, 0, 0, 1})
		serial2 = device.Serial([8]byte{0, 0, 0, 0, 0, 2})
		devices = []*device.Device{
			{Serial: serial0}, {Serial: serial1}, {Serial: serial2},
		}
		selectors = map[string][]*device.Device{
			"d00000000000": {devices[0]},
			"moon":         {devices[0]},
			"living room":  {devices[0], devices[1]}, "home": {devices[0], devices[1], devices[2]},
		}
	)

	testCases := map[string]struct {
		tokens      []string
		selectors   map[string][]*device.Device
		wantMatches map[int]*selectorMatch
	}{
		"serial": {
			tokens: []string{"set", "d00000000000", "to", "blue"},
			wantMatches: map[int]*selectorMatch{
				1: {Match: "d00000000000", Span: 1, Devices: []*device.Device{devices[0]}},
			},
		},
		"single word label": {
			tokens: []string{"set", "moon", "to", "red"},
			wantMatches: map[int]*selectorMatch{
				1: {Match: "moon", Span: 1, Devices: []*device.Device{devices[0]}},
			},
		},
		"multi word group": {
			tokens: []string{"set", "living", "room", "to", "red"},
			wantMatches: map[int]*selectorMatch{
				1: {Match: "living room", Span: 2, Devices: []*device.Device{devices[0], devices[1]}},
			},
		},
		"location": {
			tokens: []string{"set", "home", "to", "red"},
			wantMatches: map[int]*selectorMatch{
				1: {Match: "home", Span: 1, Devices: []*device.Device{devices[0], devices[1], devices[2]}},
			},
		},
		"multi target": {
			tokens: []string{"set", "moon", "and", "living", "room", "to", "green"},
			wantMatches: map[int]*selectorMatch{
				1: {Match: "moon", Span: 1, Devices: []*device.Device{devices[0]}},
				3: {Match: "living room", Span: 2, Devices: []*device.Device{devices[0], devices[1]}},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			cmdParser := &CommandParser{selectors: selectors}
			matches := cmdParser.matchEntities(tc.tokens)
			assert.Equal(t, tc.wantMatches, matches)
		})
	}
}

func TestForEachSend(t *testing.T) {
	var (
		serial0 = device.Serial([8]byte{0, 0, 0, 0, 0, 0})
		serial1 = device.Serial([8]byte{0, 0, 0, 0, 0, 1})

		msg0 = &protocol.Message{}
		msg1 = &protocol.Message{}
	)

	type call struct {
		s   device.Serial
		msg *protocol.Message
	}
	var calls []call

	cmd := Command{Targets: []device.Serial{serial0, serial1}, Msgs: []*protocol.Message{msg0, msg1}}
	cmd.ForEachSend(func(s device.Serial, msg *protocol.Message) {
		calls = append(calls, struct {
			s   device.Serial
			msg *protocol.Message
		}{s, msg})
	})

	expected := []call{
		{serial0, msg0},
		{serial1, msg0},
		{serial0, msg1},
		{serial1, msg1},
	}

	assert.Equal(t, expected, calls)
}

func Test_selectorsFromDevices(t *testing.T) {
	var (
		serial0 = device.Serial([8]byte{0, 0, 0, 0, 0, 0})
		serial1 = device.Serial([8]byte{0, 0, 0, 0, 0, 1})
		serial2 = device.Serial([8]byte{0, 0, 0, 0, 0, 2})

		device0 = device.Device{Serial: serial0, Label: "moon", Group: "tv", Location: "home"}
		device1 = device.Device{Serial: serial1, Label: "luna", Group: "living room", Location: "home"}
		device2 = device.Device{Serial: serial2, Label: "neon", Group: "living room", Location: "home"}
	)

	testCases := map[string]struct {
		devices []device.Device
		want    map[string][]*device.Device
	}{
		"single": {
			devices: []device.Device{device0},
			want: map[string][]*device.Device{
				"000000000000": {&device0},
				"moon":         {&device0},
				"tv":           {&device0},
				"home":         {&device0},
				"all":          {&device0},
			},
		},
		"multiple devices": {
			devices: []device.Device{device0, device1, device2},
			want: map[string][]*device.Device{
				"000000000000": {&device0},
				"moon":         {&device0},
				"000000000001": {&device1},
				"luna":         {&device1},
				"000000000002": {&device2},
				"neon":         {&device2},
				"tv":           {&device0},
				"living room":  {&device1, &device2},
				"home":         {&device0, &device1, &device2},
				"all":          {&device0, &device1, &device2},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := selectorsFromDevices(tc.devices)
			assert.Equal(t, tc.want, got)
		})
	}
}
