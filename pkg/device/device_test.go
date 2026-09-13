package device

import (
	"math"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/enums"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	"github.com/stretchr/testify/assert"
)

func TestSetProductInfo(t *testing.T) {
	testCases := map[string]struct {
		pid  uint32
		want *Device
	}{
		"Single zone light (white only)": {
			pid: 88,
			want: &Device{
				ProductID:     88,
				RegistryName:  "LIFX White",
				RegistryKnown: true,
				LightType:     LightTypeSingleZone,
				ColorProperties: ColorProperties{
					TemperatureRange: TemperatureRange{Min: 2700, Max: 2700},
				},
			},
		},
		"Single zone light": {
			pid: 97,
			want: &Device{
				ProductID:     97,
				RegistryName:  "LIFX Colour A19 1200lm",
				RegistryKnown: true,
				LightType:     LightTypeSingleZone,
				ColorProperties: ColorProperties{
					HasColor:         true,
					TemperatureRange: TemperatureRange{Min: 1500, Max: 9000},
				},
			},
		},
		"Multizone light": {
			pid: 117,
			want: &Device{
				ProductID:     117,
				RegistryName:  "LIFX Z",
				RegistryKnown: true,
				LightType:     LightTypeMultiZone,
				ColorProperties: ColorProperties{
					HasColor:         true,
					TemperatureRange: TemperatureRange{Min: 1500, Max: 9000},
				},
			},
		},
		"Matrix light": {
			pid: 55,
			want: &Device{
				ProductID:     55,
				RegistryName:  "LIFX Tile",
				RegistryKnown: true,
				LightType:     LightTypeMatrix,
				ColorProperties: ColorProperties{
					HasColor:         true,
					TemperatureRange: TemperatureRange{Min: 2500, Max: 9000},
				},
			},
		},
		"Switch": {
			pid: 89,
			want: &Device{
				ProductID:     89,
				RegistryName:  "LIFX Switch",
				RegistryKnown: true,
				Type:          DeviceTypeSwitch,
			},
		},
		"Hybrid": {
			pid: 219,
			want: &Device{
				ProductID:     219,
				RegistryName:  "LIFX Luna",
				RegistryKnown: true,
				Type:          DeviceTypeHybrid,
				LightType:     LightTypeMatrix,
				ColorProperties: ColorProperties{
					HasColor:         true,
					TemperatureRange: TemperatureRange{Min: 1500, Max: 9000},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			d := &Device{}
			d.SetProductInfo(tc.pid)
			assert.Equal(t, tc.want, d)
		})
	}
}

func TestSetProductInfoForUnknownProduct(t *testing.T) {
	d := &Device{}
	d.SetProductInfo(999999)

	want := &Device{
		ProductID:       999999,
		Type:            DeviceTypeLight,
		LightType:       LightTypeSingleZone,
		ColorProperties: defaultColorProperties(),
	}
	assert.Equal(t, want, d)
}

func TestSortDevices(t *testing.T) {
	var (
		serial0 = Serial([8]byte{0, 0, 0, 0, 0, 0, 0, 0})
		serial1 = Serial([8]byte{1, 0, 0, 0, 0, 0, 0, 0})
	)
	testCases := map[string]struct {
		devices []Device
		want    []Device
	}{
		"devices with different label": {
			devices: []Device{{Serial: serial0, Label: "B"}, {Serial: serial1, Label: "A"}},
			want:    []Device{{Serial: serial1, Label: "A"}, {Serial: serial0, Label: "B"}},
		},
		"devices with same label": {
			devices: []Device{{Serial: serial1, Label: "A"}, {Serial: serial0, Label: "A"}},
			want:    []Device{{Serial: serial0, Label: "A"}, {Serial: serial1, Label: "A"}},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			SortDevices(tc.devices)
			assert.Equal(t, tc.want, tc.devices)
		})
	}
}

func TestSetMatrixPropertiesInfersUnknownProductLightType(t *testing.T) {
	d := &Device{ProductID: 999999}

	updated := d.SetMatrixProperties(&packets.TileStateDeviceChain{
		TileDevices:      [16]packets.TileStateDevice{{Width: 8, Height: 8}},
		TileDevicesCount: 1,
	})

	assert.True(t, updated)
	assert.Equal(t, LightTypeMatrix, d.LightType)
}

func TestSetMultizonePropertiesInfersUnknownProductLightType(t *testing.T) {
	d := &Device{ProductID: 999999}

	updated := d.SetMultizoneProperties(&packets.MultiZoneExtendedStateMultiZone{
		Count:       2,
		ColorsCount: 2,
		Colors:      [82]packets.LightHsbk{{Hue: 1}, {Hue: 2}},
	})

	assert.True(t, updated)
	assert.Equal(t, LightTypeMultiZone, d.LightType)
}

func TestSetMultizonePropertiesReportsOnlyChanges(t *testing.T) {
	state := &packets.MultiZoneExtendedStateMultiZone{
		Count:       2,
		ColorsCount: 2,
		Colors:      [82]packets.LightHsbk{{Hue: 1}, {Hue: 2}},
	}
	d := &Device{}

	assert.True(t, d.SetMultizoneProperties(state))
	assert.False(t, d.SetMultizoneProperties(state))

	state.Colors[1].Hue = 3
	assert.True(t, d.SetMultizoneProperties(state))
	assert.Equal(t, uint16(3), d.MultizoneProperties.Zones[1].Hue)
}

func TestCapabilityStateDoesNotOverrideKnownRegistryLightType(t *testing.T) {
	d := &Device{}
	d.SetProductInfo(97)

	d.SetMatrixProperties(&packets.TileStateDeviceChain{
		TileDevices:      [16]packets.TileStateDevice{{Width: 8, Height: 8}},
		TileDevicesCount: 1,
	})
	assert.Equal(t, LightTypeSingleZone, d.LightType)

	d.SetMultizoneProperties(&packets.MultiZoneExtendedStateMultiZone{
		Count:       2,
		ColorsCount: 2,
		Colors:      [82]packets.LightHsbk{{Hue: 1}, {Hue: 2}},
	})
	assert.Equal(t, LightTypeSingleZone, d.LightType)
}

func TestSetMatrixProperties(t *testing.T) {
	emptyZoneSlice64 := make([]packets.LightHsbk, 64)
	emptyZoneSlice128 := make([]packets.LightHsbk, 128)

	tests := map[string]struct {
		device      *Device
		msg         *packets.TileStateDeviceChain
		want        *Device
		wantUpdated bool
	}{
		"bad message": {
			device: &Device{},
			msg:    &packets.TileStateDeviceChain{},
			want:   &Device{},
		},
		"does not update if unchanged": {
			device: &Device{
				MatrixProperties: MatrixProperties{
					Height: 8, Width: 8, ChainLength: 2, NZones: 64, StatePackets: 1,
					ChainZones:        [][]packets.LightHsbk{emptyZoneSlice64, emptyZoneSlice64},
					ChainOrientations: []Orientation{OrientationRightSideUp, OrientationRightSideUp},
				},
			},
			msg: &packets.TileStateDeviceChain{
				TileDevices:      [16]packets.TileStateDevice{{Width: 8, Height: 8}, {Width: 8, Height: 8}},
				TileDevicesCount: 2,
			},
			want: &Device{
				MatrixProperties: MatrixProperties{
					Height: 8, Width: 8, ChainLength: 2, NZones: 64, StatePackets: 1,
					ChainZones:        [][]packets.LightHsbk{emptyZoneSlice64, emptyZoneSlice64},
					ChainOrientations: []Orientation{OrientationRightSideUp, OrientationRightSideUp},
				},
			},
		},
		"sets properties (zones = 64)": {
			device: &Device{},
			msg: &packets.TileStateDeviceChain{
				TileDevices:      [16]packets.TileStateDevice{{Width: 8, Height: 8}, {Width: 8, Height: 8}},
				TileDevicesCount: 2,
			},
			want: &Device{
				MatrixProperties: MatrixProperties{
					Height: 8, Width: 8, ChainLength: 2, NZones: 64, StatePackets: 1,
					ChainZones:        [][]packets.LightHsbk{emptyZoneSlice64, emptyZoneSlice64},
					ChainOrientations: []Orientation{OrientationRightSideUp, OrientationRightSideUp},
				},
			},
			wantUpdated: true,
		},
		"sets properties (zones < 64)": {
			device: &Device{},
			msg: &packets.TileStateDeviceChain{
				TileDevices:      [16]packets.TileStateDevice{{Width: 7, Height: 5}, {Width: 7, Height: 5}},
				TileDevicesCount: 2,
			},
			want: &Device{
				MatrixProperties: MatrixProperties{
					Height: 5, Width: 7, ChainLength: 2, NZones: 35, StatePackets: 1,
					ChainZones:        [][]packets.LightHsbk{emptyZoneSlice64[:35], emptyZoneSlice64[:35]},
					ChainOrientations: []Orientation{OrientationRightSideUp, OrientationRightSideUp},
				},
			},
			wantUpdated: true,
		},
		"sets properties (zones > 64)": {
			device: &Device{},
			msg: &packets.TileStateDeviceChain{
				TileDevices:      [16]packets.TileStateDevice{{Width: 16, Height: 8}, {Width: 16, Height: 8}},
				TileDevicesCount: 2,
			},
			want: &Device{
				MatrixProperties: MatrixProperties{
					Height: 8, Width: 16, ChainLength: 2, NZones: 128, StatePackets: 2,
					ChainZones:        [][]packets.LightHsbk{emptyZoneSlice128, emptyZoneSlice128},
					ChainOrientations: []Orientation{OrientationRightSideUp, OrientationRightSideUp},
				},
			},
			wantUpdated: true,
		},
		"sets properties when start at offset": {
			device: &Device{},
			msg: &packets.TileStateDeviceChain{
				StartIndex:       2,
				TileDevices:      [16]packets.TileStateDevice{{}, {}, {Width: 8, Height: 8}},
				TileDevicesCount: 1,
			},
			want: &Device{
				MatrixProperties: MatrixProperties{
					Height: 8, Width: 8, ChainLength: 1, NZones: 64, StatePackets: 1,
					ChainZones:        [][]packets.LightHsbk{emptyZoneSlice64},
					ChainOrientations: []Orientation{OrientationRightSideUp},
				},
			},
			wantUpdated: true,
		},
		"adds tile to existing chain": {
			device: &Device{MatrixProperties: MatrixProperties{ChainZones: [][]packets.LightHsbk{emptyZoneSlice64}}},
			msg: &packets.TileStateDeviceChain{
				StartIndex:       0,
				TileDevices:      [16]packets.TileStateDevice{{Width: 8, Height: 8}, {Width: 8, Height: 8}},
				TileDevicesCount: 2,
			},
			want: &Device{
				MatrixProperties: MatrixProperties{
					Height: 8, Width: 8, ChainLength: 2, NZones: 64, StatePackets: 1,
					ChainZones:        [][]packets.LightHsbk{emptyZoneSlice64, emptyZoneSlice64},
					ChainOrientations: []Orientation{OrientationRightSideUp, OrientationRightSideUp},
				},
			},
			wantUpdated: true,
		},
		"deletes tile from existing chain": {
			device: &Device{MatrixProperties: MatrixProperties{ChainZones: [][]packets.LightHsbk{emptyZoneSlice64, emptyZoneSlice64}}},
			msg: &packets.TileStateDeviceChain{
				StartIndex:       0,
				TileDevices:      [16]packets.TileStateDevice{{Width: 8, Height: 8}},
				TileDevicesCount: 1,
			},
			want: &Device{
				MatrixProperties: MatrixProperties{
					Height: 8, Width: 8, ChainLength: 1, NZones: 64, StatePackets: 1,
					ChainZones:        [][]packets.LightHsbk{emptyZoneSlice64},
					ChainOrientations: []Orientation{OrientationRightSideUp},
				},
			},
			wantUpdated: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			updated := tc.device.SetMatrixProperties(tc.msg)
			assert.Equal(t, tc.want, tc.device)
			assert.Equal(t, tc.wantUpdated, updated)
		})
	}
}

func TestSetMatrixState(t *testing.T) {
	emptyZoneSlice := func() []packets.LightHsbk { return make([]packets.LightHsbk, 64) }
	color0 := packets.LightHsbk{Hue: 180, Saturation: math.MaxUint16, Brightness: math.MaxUint16, Kelvin: 3500}
	packet0 := [64]packets.LightHsbk{color0}

	packet1 := [64]packets.LightHsbk{color0, color0, color0}
	zones0 := make([]packets.LightHsbk, 128)
	copy(zones0, packet0[:])
	copy(zones0[64:], packet1[:])

	tests := map[string]struct {
		device      *Device
		msgs        []*packets.TileState64
		want        *Device
		wantUpdated []bool
	}{
		"device has no matrix properties": {
			device: &Device{},
			msgs: []*packets.TileState64{
				{TileIndex: 0, Colors: [64]packets.LightHsbk{}},
			},
			want:        &Device{},
			wantUpdated: []bool{false},
		},
		"does not updated if unchanged": {
			device: &Device{
				MatrixProperties: MatrixProperties{
					Height: 8, Width: 8, ChainLength: 2,
					ChainZones: [][]packets.LightHsbk{packet0[:], emptyZoneSlice()},
				},
			},
			msgs: []*packets.TileState64{
				{TileIndex: 0, Colors: [64]packets.LightHsbk{color0}},
			},
			want: &Device{
				MatrixProperties: MatrixProperties{
					Height: 8, Width: 8, ChainLength: 2,
					ChainZones: [][]packets.LightHsbk{packet0[:], emptyZoneSlice()},
				},
			},
			wantUpdated: []bool{false},
		},
		"sets matrix zones": {
			device: &Device{
				MatrixProperties: MatrixProperties{
					Height: 8, Width: 8, ChainLength: 2, StatePackets: 1,
					ChainZones: [][]packets.LightHsbk{emptyZoneSlice(), emptyZoneSlice()},
				},
			},
			msgs: []*packets.TileState64{
				{TileIndex: 0, Colors: [64]packets.LightHsbk{color0}},
			},
			want: &Device{
				MatrixProperties: MatrixProperties{
					Height: 8, Width: 8, ChainLength: 2, StatePackets: 1,
					ChainZones: [][]packets.LightHsbk{packet0[:], emptyZoneSlice()},
				},
			},
			wantUpdated: []bool{true},
		},
		"sets matrix state at offset": {
			device: &Device{
				MatrixProperties: MatrixProperties{
					Height: 8, Width: 8, ChainLength: 2,
					ChainZones: [][]packets.LightHsbk{emptyZoneSlice(), emptyZoneSlice()},
				},
			},
			msgs: []*packets.TileState64{
				{TileIndex: 1, Colors: [64]packets.LightHsbk{color0}},
			},
			want: &Device{
				MatrixProperties: MatrixProperties{
					Height: 8, Width: 8, ChainLength: 2,
					ChainZones: [][]packets.LightHsbk{emptyZoneSlice(), packet0[:]},
				},
			},
			wantUpdated: []bool{true},
		},
		"sets matrix with more than 64 zones": {
			device: &Device{
				MatrixProperties: MatrixProperties{
					Height: 8, Width: 16, ChainLength: 1,
					ChainZones: [][]packets.LightHsbk{make([]packets.LightHsbk, 128)},
				},
			},
			msgs: []*packets.TileState64{
				{TileIndex: 0, Colors: packet0},
				{TileIndex: 0, Colors: packet1, Rect: packets.TileBufferRect{Y: 4}},
			},
			want: &Device{
				MatrixProperties: MatrixProperties{
					Height: 8, Width: 16, ChainLength: 1,
					ChainZones: [][]packets.LightHsbk{zones0},
				},
			},
			wantUpdated: []bool{true, true},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var updated []bool
			for _, msg := range tc.msgs {
				got := tc.device.SetMatrixState(msg)
				updated = append(updated, got)
			}
			assert.Equal(t, tc.want, tc.device)
			assert.Equal(t, tc.wantUpdated, updated)
		})
	}
}

func TestSetMultizoneProperties(t *testing.T) {
	color0 := packets.LightHsbk{Hue: 0, Saturation: math.MaxUint16, Brightness: math.MaxUint16, Kelvin: 3500}
	withColors := func(index, count int, colors ...packets.LightHsbk) []packets.LightHsbk {
		zones := make([]packets.LightHsbk, count)
		copy(zones[index:], colors)
		return zones
	}

	tests := map[string]struct {
		device      *Device
		msgs        []*packets.MultiZoneExtendedStateMultiZone
		want        *Device
		wantUpdated []bool
	}{
		"bad message": {
			device:      &Device{},
			msgs:        []*packets.MultiZoneExtendedStateMultiZone{{}},
			want:        &Device{},
			wantUpdated: []bool{false},
		},
		"start index greater than zones": {
			device: &Device{MultizoneProperties: MultizoneProperties{Zones: make([]packets.LightHsbk, 8)}},
			msgs: []*packets.MultiZoneExtendedStateMultiZone{
				{Index: 9, Count: 8, ColorsCount: 1, Colors: [82]packets.LightHsbk{color0}},
			},
			want: &Device{
				MultizoneProperties: MultizoneProperties{
					Zones: make([]packets.LightHsbk, 8),
				},
			},
			wantUpdated: []bool{false},
		},
		"sets properties with single message": {
			device: &Device{},
			msgs: []*packets.MultiZoneExtendedStateMultiZone{
				{Index: 0, Count: 24, ColorsCount: 1, Colors: [82]packets.LightHsbk{color0}},
			},
			want: &Device{
				MultizoneProperties: MultizoneProperties{
					Zones: withColors(0, 24, color0),
				},
			},
			wantUpdated: []bool{true},
		},
		"sets properties with single message at offset": {
			device: &Device{},
			msgs: []*packets.MultiZoneExtendedStateMultiZone{
				{Index: 23, Count: 24, ColorsCount: 1, Colors: [82]packets.LightHsbk{color0}},
			},
			want: &Device{
				MultizoneProperties: MultizoneProperties{
					Zones: withColors(23, 24, color0),
				},
			},
			wantUpdated: []bool{true},
		},
		"sets properties with multiple messages": {
			device: &Device{},
			msgs: []*packets.MultiZoneExtendedStateMultiZone{
				{Index: 81, Count: 120, ColorsCount: 2, Colors: [82]packets.LightHsbk{color0, color0}},
				{Index: 83, Count: 120, ColorsCount: 1, Colors: [82]packets.LightHsbk{color0}},
			},
			want: &Device{
				MultizoneProperties: MultizoneProperties{
					Zones: withColors(81, 120, color0, color0, color0),
				},
			},
			wantUpdated: []bool{true, true},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var updated []bool
			for _, msg := range tc.msgs {
				got := tc.device.SetMultizoneProperties(msg)
				updated = append(updated, got)
			}
			assert.Equal(t, tc.want, tc.device)
			assert.Equal(t, tc.wantUpdated, updated)
		})
	}
}

func TestSetButtons(t *testing.T) {
	button0 := Button{
		Actions: []packets.ButtonAction{
			{
				Gesture:    enums.ButtonGestureBUTTONGESTUREHOLD,
				TargetType: enums.ButtonTargetTypeBUTTONTARGETTYPEBRIGHTNESSDOWNDEVICE,
				Target:     packets.ButtonTarget{},
			},
		},
	}
	button1 := Button{
		Actions: []packets.ButtonAction{
			{
				Gesture:    enums.ButtonGestureBUTTONGESTUREHOLD,
				TargetType: enums.ButtonTargetTypeBUTTONTARGETTYPEPOWERONRELAYS,
				Target:     packets.ButtonTarget{},
			},
			{
				Gesture:    enums.ButtonGestureBUTTONGESTUREHOLDTOUCH,
				TargetType: enums.ButtonTargetTypeBUTTONTARGETTYPEBRIGHTNESSDOWNDEVICE,
				Target:     packets.ButtonTarget{},
			},
		},
	}
	button2 := Button{
		Actions: []packets.ButtonAction{
			{
				Gesture:    enums.ButtonGestureBUTTONGESTUREHOLD,
				TargetType: enums.ButtonTargetTypeBUTTONTARGETTYPEBRIGHTNESSUPDEVICE,
				Target:     packets.ButtonTarget{},
			},
			{
				Gesture:    enums.ButtonGestureBUTTONGESTUREHOLDRELEASE,
				TargetType: enums.ButtonTargetTypeBUTTONTARGETTYPEPOWEROFFGROUP,
				Target:     packets.ButtonTarget{},
			},
		},
	}

	tests := map[string]struct {
		device      *Device
		msg         *packets.ButtonState
		want        *Device
		wantUpdated bool
	}{
		"bad message": {
			device:      &Device{},
			msg:         &packets.ButtonState{},
			want:        &Device{},
			wantUpdated: false,
		},
		"no change": {
			device: &Device{Buttons: []Button{button0, button1}},
			msg: &packets.ButtonState{
				ButtonsCount: 2, Buttons: [8]packets.Button{
					{ActionsCount: 1, Actions: [5]packets.ButtonAction{button0.Actions[0]}},
					{ActionsCount: 2, Actions: [5]packets.ButtonAction{button1.Actions[0], button1.Actions[1]}},
				},
			},
			want:        &Device{Buttons: []Button{button0, button1}},
			wantUpdated: false,
		},
		"add button": {
			device: &Device{Buttons: []Button{button0}},
			msg: &packets.ButtonState{
				ButtonsCount: 2, Buttons: [8]packets.Button{
					{ActionsCount: 1, Actions: [5]packets.ButtonAction{button0.Actions[0]}},
					{ActionsCount: 2, Actions: [5]packets.ButtonAction{button1.Actions[0], button1.Actions[1]}},
				},
			},
			want:        &Device{Buttons: []Button{button0, button1}},
			wantUpdated: true,
		},
		"remove button": {
			device: &Device{Buttons: []Button{button0, button1}},
			msg: &packets.ButtonState{
				ButtonsCount: 1, Buttons: [8]packets.Button{
					{ActionsCount: 1, Actions: [5]packets.ButtonAction{button0.Actions[0]}},
				},
			},
			want:        &Device{Buttons: []Button{button0}},
			wantUpdated: true,
		},
		"updates button": {
			device: &Device{Buttons: []Button{button0, button2}},
			msg: &packets.ButtonState{
				ButtonsCount: 2, Buttons: [8]packets.Button{
					{ActionsCount: 1, Actions: [5]packets.ButtonAction{button0.Actions[0]}},
					{ActionsCount: 2, Actions: [5]packets.ButtonAction{button1.Actions[0], button1.Actions[1]}},
				},
			},
			want:        &Device{Buttons: []Button{button0, button1}},
			wantUpdated: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			updated := tc.device.SetButtons(tc.msg)
			assert.Equal(t, tc.want, tc.device)
			assert.Equal(t, tc.wantUpdated, updated)
		})
	}
}

func TestSetRelayPower(t *testing.T) {
	tests := map[string]struct {
		device      *Device
		msg         *packets.RelayStatePower
		want        *Device
		wantUpdated bool
	}{
		"adds relay": {
			device:      &Device{},
			msg:         &packets.RelayStatePower{RelayIndex: 1, Level: math.MaxUint16},
			want:        &Device{Relays: []Relay{{Index: 1, PoweredOn: true}}},
			wantUpdated: true,
		},
		"adds relay zero when reported": {
			device:      &Device{},
			msg:         &packets.RelayStatePower{RelayIndex: 0, Level: math.MaxUint16},
			want:        &Device{Relays: []Relay{{Index: 0, PoweredOn: true}}},
			wantUpdated: true,
		},
		"no change": {
			device:      &Device{Relays: []Relay{{Index: 1, PoweredOn: true}}},
			msg:         &packets.RelayStatePower{RelayIndex: 1, Level: math.MaxUint16},
			want:        &Device{Relays: []Relay{{Index: 1, PoweredOn: true}}},
			wantUpdated: false,
		},
		"updates relay": {
			device:      &Device{Relays: []Relay{{Index: 1, PoweredOn: true}}},
			msg:         &packets.RelayStatePower{RelayIndex: 1, Level: 0},
			want:        &Device{Relays: []Relay{{Index: 1}}},
			wantUpdated: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			updated := tc.device.SetRelayPower(tc.msg)
			assert.Equal(t, tc.want, tc.device)
			assert.Equal(t, tc.wantUpdated, updated)
		})
	}
}

func TestSetButtonConfig(t *testing.T) {
	msg := &packets.ButtonStateConfig{
		HapticDurationMs: 250,
		BacklightOnColor: packets.ButtonBacklightHsbk{
			Hue: math.MaxUint16, Saturation: math.MaxUint16, Brightness: math.MaxUint16, Kelvin: 3500,
		},
		BacklightOffColor: packets.ButtonBacklightHsbk{Kelvin: 3500},
	}
	want := &Device{ButtonConfig: ButtonConfig{
		HapticDurationMs:  250,
		BacklightOnColor:  Color{Hue: 360, Saturation: 100, Brightness: 100, Kelvin: 3500},
		BacklightOffColor: Color{Kelvin: 3500},
	}, ButtonConfigKnown: true}

	d := &Device{}
	assert.True(t, d.SetButtonConfig(msg))
	assert.Equal(t, want, d)
	assert.False(t, d.SetButtonConfig(msg))
}

func TestSetMultizoneEffect(t *testing.T) {
	state := &packets.MultiZoneStateEffect{Settings: packets.MultiZoneEffectSettings{
		Instanceid: 42,
		Type:       enums.MultiZoneEffectTypeMULTIZONEEFFECTTYPEMOVE,
		Speed:      2500,
		Duration:   uint64(30 * time.Second),
		Parameter:  packets.MultiZoneEffectParameter{Parameter1: 1},
	}}
	d := &Device{}

	assert.True(t, d.SetMultizoneEffect(state))
	assert.Equal(t, MultizoneEffect{
		Known: true, Type: MultizoneEffectTypeMove, InstanceID: 42,
		Speed: 2500 * time.Millisecond, RemainingDuration: 30 * time.Second, Direction: EffectDirectionForward,
	}, d.MultizoneProperties.Effect)
	assert.True(t, d.MultizoneProperties.Effect.Running())
	assert.False(t, d.SetMultizoneEffect(state))

	state.Settings.Type = enums.MultiZoneEffectTypeMULTIZONEEFFECTTYPEOFF
	assert.True(t, d.SetMultizoneEffect(state))
	assert.False(t, d.MultizoneProperties.Effect.Running())
}

func TestSetMatrixEffect(t *testing.T) {
	state := &packets.TileStateEffect{Settings: packets.TileEffectSettings{
		Instanceid:   84,
		Type:         enums.TileEffectTypeTILEEFFECTTYPESKY,
		Speed:        5000,
		Duration:     uint64(time.Minute),
		Parameter:    packets.TileEffectParameter{Parameter0: uint32(enums.TileEffectSkyTypeTILEEFFECTSKYTYPECLOUDS)},
		PaletteCount: 1,
		Palette:      [16]packets.LightHsbk{{Hue: math.MaxUint16, Saturation: math.MaxUint16, Brightness: math.MaxUint16, Kelvin: 3500}},
	}}
	d := &Device{}

	assert.True(t, d.SetMatrixEffect(state))
	assert.Equal(t, MatrixEffect{
		Known: true, Type: MatrixEffectTypeSky, SkyType: MatrixEffectSkyTypeClouds, InstanceID: 84,
		Speed: 5 * time.Second, RemainingDuration: time.Minute,
		Palette: []Color{{Hue: 360, Saturation: 100, Brightness: 100, Kelvin: 3500}},
	}, d.MatrixProperties.Effect)
	assert.True(t, d.MatrixProperties.Effect.Running())
	assert.False(t, d.SetMatrixEffect(state))

	state.Settings.Palette[0].Hue = 0
	assert.True(t, d.SetMatrixEffect(state))
	assert.Equal(t, float64(0), d.MatrixProperties.Effect.Palette[0].Hue)
}

func TestStateMessagesForSwitch(t *testing.T) {
	switchDevice := &Device{Type: DeviceTypeSwitch, Buttons: []Button{{}, {}}}
	assert.Equal(t, []packets.Payload{
		&packets.RelayGetPower{RelayIndex: 0},
		&packets.RelayGetPower{RelayIndex: 1},
	}, payloads(switchDevice.HighFreqStateMessages()))

	assert.Contains(t, payloads(switchDevice.LowFreqStateMessages()), &packets.ButtonGet{})
	assert.Contains(t, payloads(switchDevice.LowFreqStateMessages()), &packets.ButtonGetConfig{})
}

func TestStateMessagesIncludeCapabilitySpecificEffectState(t *testing.T) {
	multizone := &Device{LightType: LightTypeMultiZone}
	assert.Contains(t, payloads(multizone.HighFreqStateMessages()), &packets.MultiZoneGetEffect{})

	matrix := &Device{LightType: LightTypeMatrix}
	assert.Contains(t, payloads(matrix.HighFreqStateMessages()), &packets.TileGetEffect{})

	singleZone := &Device{LightType: LightTypeSingleZone}
	for _, payload := range payloads(singleZone.HighFreqStateMessages()) {
		_, multizoneEffect := payload.(*packets.MultiZoneGetEffect)
		_, matrixEffect := payload.(*packets.TileGetEffect)
		assert.False(t, multizoneEffect || matrixEffect)
	}
}

func TestStateMessagesForHybridDoesNotPollRelays(t *testing.T) {
	hybrid := &Device{Type: DeviceTypeHybrid, LightType: LightTypeMatrix, Buttons: []Button{{}, {}}, MatrixProperties: MatrixProperties{ChainLength: 1, StatePackets: 1, Width: 8}}

	for _, payload := range payloads(hybrid.HighFreqStateMessages()) {
		_, ok := payload.(*packets.RelayGetPower)
		assert.False(t, ok)
	}
	assert.Contains(t, payloads(hybrid.LowFreqStateMessages()), &packets.ButtonGet{})
	assert.Contains(t, payloads(hybrid.LowFreqStateMessages()), &packets.ButtonGetConfig{})
}

func payloads(msgs []*protocol.Message) []packets.Payload {
	out := make([]packets.Payload, 0, len(msgs))
	for _, msg := range msgs {
		out = append(out, msg.Payload)
	}
	return out
}
