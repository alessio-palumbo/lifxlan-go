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
			{Serial: serial0, Label: "moon", Group: "tv", Location: "home", Color: device.Color{Brightness: 5, Saturation: 40, Kelvin: 3500}, ColorProperties: device.ColorProperties{TemperatureRange: device.TemperatureRange{Min: 2500, Max: 9000}}},
			{Serial: serial1, Label: "luna", Group: "living room", Location: "home", Color: device.Color{Brightness: 50, Saturation: 60, Kelvin: 3500}, ColorProperties: device.ColorProperties{TemperatureRange: device.TemperatureRange{Min: 2500, Max: 9000}}},
			{Serial: serial2, Label: "neon", Group: "living room", Location: "home", Color: device.Color{Brightness: 95, Saturation: 95, Kelvin: 8700}, ColorProperties: device.ColorProperties{TemperatureRange: device.TemperatureRange{Min: 2500, Max: 9000}}},
			{Serial: serial3, Label: "filo", Group: "tv", Location: "home", Color: device.Color{Brightness: 20, Saturation: 5, Kelvin: 2600}, ColorProperties: device.ColorProperties{TemperatureRange: device.TemperatureRange{Min: 2500, Max: 9000}}},
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
							Cycles: 1, Period: 1, SetHue: true, SetSaturation: true,
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
							Cycles: 1, Period: 1, SetHue: true, SetSaturation: true,
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
							Cycles: 1, Period: 1, SetHue: true, SetSaturation: true,
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
							Cycles: 1, Period: 1, SetHue: true, SetSaturation: true,
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
							Cycles: 1, Period: 1, SetHue: true, SetSaturation: true,
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
							Cycles: 1, Period: 1, SetHue: true, SetSaturation: true,
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
							Cycles: 1, Period: 1, SetHue: true, SetSaturation: true,
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
							Cycles: 1, Period: 1, SetHue: true, SetSaturation: true, SetBrightness: true,
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
							Cycles: 1, Period: 1, SetHue: true, SetSaturation: true, SetBrightness: true,
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
							Cycles: 1, Period: 1, SetHue: true, SetSaturation: true, SetBrightness: true,
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
							Cycles: 1, Period: 1, SetHue: true, SetSaturation: true,
							Color: packets.LightHsbk{Hue: 21845, Saturation: 65535},
						}),
					},
				},
				{
					Targets: []device.Serial{serial0},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1, SetBrightness: true,
							Color: packets.LightHsbk{Brightness: 19661},
						}),
					},
				},
			},
		},
		"relative brightness verb before selector with explicit amount": {
			input: "dim luna 30%",
			want: []Command{
				{
					Targets: []device.Serial{serial1},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1, SetBrightness: true,
							Color: packets.LightHsbk{Brightness: 13107},
						}),
					},
				},
			},
		},
		"relative brightness adjective after selector with default amount": {
			input: "luna brighter",
			want: []Command{
				{
					Targets: []device.Serial{serial1},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1, SetBrightness: true,
							Color: packets.LightHsbk{Brightness: 39321},
						}),
					},
				},
			},
		},
		"relative brightness splits commands per target": {
			input: "living room brighter 10%",
			want: []Command{
				{
					Targets: []device.Serial{serial1},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1, SetBrightness: true,
							Color: packets.LightHsbk{Brightness: 39321},
						}),
					},
				},
				{
					Targets: []device.Serial{serial2},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1, SetBrightness: true,
							Color: packets.LightHsbk{Brightness: 65535},
						}),
					},
				},
			},
		},
		"relative brightness clamps above zero": {
			input: "moon dim 10%",
			want: []Command{
				{
					Targets: []device.Serial{serial0},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1, SetBrightness: true,
							Color: packets.LightHsbk{Brightness: 655},
						}),
					},
				},
			},
		},
		"relative temperature warmer": {
			input: "luna warmer",
			want: []Command{
				{
					Targets: []device.Serial{serial1},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1, SetKelvin: true,
							Color: packets.LightHsbk{Kelvin: 4000},
						}),
					},
				},
			},
		},
		"white sets saturation to zero": {
			input: "luna white",
			want: []Command{
				{
					Targets: []device.Serial{serial1},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1, SetSaturation: true,
							Color: packets.LightHsbk{Saturation: 0},
						}),
					},
				},
			},
		},
		"warm white sets visible temperature": {
			input: "luna warm white",
			want: []Command{
				{
					Targets: []device.Serial{serial1},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1, SetSaturation: true, SetKelvin: true,
							Color: packets.LightHsbk{Saturation: 0, Kelvin: 2700},
						}),
					},
				},
			},
		},
		"daylight sets visible temperature": {
			input: "luna daylight",
			want: []Command{
				{
					Targets: []device.Serial{serial1},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1, SetSaturation: true, SetKelvin: true,
							Color: packets.LightHsbk{Saturation: 0, Kelvin: 5600},
						}),
					},
				},
			},
		},
		"styled pastel color": {
			input: "luna pastel blue",
			want: []Command{
				{
					Targets: []device.Serial{serial1},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1, SetHue: true, SetSaturation: true,
							Color: packets.LightHsbk{Hue: device.ConvertExternalToDeviceValue(250, 360), Saturation: device.ConvertExternalToDeviceValue(35, 100)},
						}),
					},
				},
			},
		},
		"styled soft color with brightness": {
			input: "luna soft pink brightness 40%",
			want: []Command{
				{
					Targets: []device.Serial{serial1},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1, SetHue: true, SetSaturation: true, SetBrightness: true,
							Color: packets.LightHsbk{Hue: device.ConvertExternalToDeviceValue(325, 360), Saturation: device.ConvertExternalToDeviceValue(45, 100), Brightness: device.ConvertExternalToDeviceValue(40, 100)},
						}),
					},
				},
			},
		},
		"soft white keeps white temperature meaning": {
			input: "luna soft white",
			want: []Command{
				{
					Targets: []device.Serial{serial1},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1, SetSaturation: true, SetKelvin: true,
							Color: packets.LightHsbk{Saturation: 0, Kelvin: 3000},
						}),
					},
				},
			},
		},
		"relative temperature cooler with explicit amount clamps to range": {
			input: "filo cooler 500%",
			want: []Command{
				{
					Targets: []device.Serial{serial3},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1, SetKelvin: true,
							Color: packets.LightHsbk{Kelvin: 2500},
						}),
					},
				},
			},
		},
		"relative saturation softer": {
			input: "luna softer",
			want: []Command{
				{
					Targets: []device.Serial{serial1},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1, SetSaturation: true,
							Color: packets.LightHsbk{Saturation: 32768},
						}),
					},
				},
			},
		},
		"relative saturation vivid clamps to max": {
			input: "neon vivid 20%",
			want: []Command{
				{
					Targets: []device.Serial{serial2},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1, SetSaturation: true,
							Color: packets.LightHsbk{Saturation: 65535},
						}),
					},
				},
			},
		},
		"relative saturation phrase increases": {
			input: "luna more vivid",
			want: []Command{
				{
					Targets: []device.Serial{serial1},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1, SetSaturation: true,
							Color: packets.LightHsbk{Saturation: 45875},
						}),
					},
				},
			},
		},
		"relative saturation phrase with explicit amount": {
			input: "luna less intense 20%",
			want: []Command{
				{
					Targets: []device.Serial{serial1},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1, SetSaturation: true,
							Color: packets.LightHsbk{Saturation: 26214},
						}),
					},
				},
			},
		},
		"relative brightness phrase increases": {
			input: "luna more bright",
			want: []Command{
				{
					Targets: []device.Serial{serial1},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1, SetBrightness: true,
							Color: packets.LightHsbk{Brightness: 39321},
						}),
					},
				},
			},
		},
		"relative temperature phrase decreases warmth": {
			input: "luna less warm",
			want: []Command{
				{
					Targets: []device.Serial{serial1},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1, SetKelvin: true,
							Color: packets.LightHsbk{Kelvin: 3000},
						}),
					},
				},
			},
		},
		"relative temperature phrase reverses cooling": {
			input: "luna less cool 1000",
			want: []Command{
				{
					Targets: []device.Serial{serial1},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1, SetKelvin: true,
							Color: packets.LightHsbk{Kelvin: 4500},
						}),
					},
				},
			},
		},
		"relative saturation phrase reverses pastel": {
			input: "filo less pastel",
			want: []Command{
				{
					Targets: []device.Serial{serial3},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1, SetSaturation: true,
							Color: packets.LightHsbk{Saturation: 9830},
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
							Cycles: 1, Period: 1, SetHue: true, SetSaturation: true, SetKelvin: true,
							Color: packets.LightHsbk{Hue: 32768, Saturation: 6554, Kelvin: 4000},
						}),
						protocol.NewMessage(&packets.DeviceSetPower{Level: 0}),
					},
				},
				{
					Targets: []device.Serial{serial1},
					Msgs: []*protocol.Message{
						protocol.NewMessage(&packets.LightSetWaveformOptional{
							Cycles: 1, Period: 1, SetBrightness: true, SetKelvin: true,
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

		device0 = device.Device{Serial: serial0, Label: "mOOn", Group: "tv", Location: "Home"}
		device1 = device.Device{Serial: serial1, Label: "luna", Group: "Living Room", Location: "Home"}
		device2 = device.Device{Serial: serial2, Label: "Neon", Group: "Living Room", Location: "Home"}
	)

	testCases := map[string]struct {
		devices       []device.Device
		wantSelectors map[string][]*device.Device
		wantLabels    map[string]string
	}{
		"single": {
			devices: []device.Device{device0},
			wantSelectors: map[string][]*device.Device{
				"000000000000": {&device0},
				"moon":         {&device0},
				"tv":           {&device0},
				"home":         {&device0},
				"all":          {&device0},
			},
			wantLabels: map[string]string{
				"moon": "mOOn",
				"tv":   "tv",
				"home": "Home",
			},
		},
		"multiple devices": {
			devices: []device.Device{device0, device1, device2},
			wantSelectors: map[string][]*device.Device{
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
			wantLabels: map[string]string{
				"moon":        "mOOn",
				"luna":        "luna",
				"neon":        "Neon",
				"tv":          "tv",
				"living room": "Living Room",
				"home":        "Home",
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			cmdParser := &CommandParser{}
			cmdParser.selectorsFromDevices(tc.devices)
			assert.Equal(t, tc.wantSelectors, cmdParser.selectors)
			assert.Equal(t, tc.wantLabels, cmdParser.selectorsLabels)
		})
	}
}
