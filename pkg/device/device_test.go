package device

import (
	"math"
	"testing"

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
				ProductID:    88,
				RegistryName: "LIFX Mini White",
				LightType:    LightTypeSingleZone,
				ColorProperties: ColorProperties{
					TemperatureRange: TemperatureRange{Min: 2700, Max: 2700},
				},
			},
		},
		"Single zone light": {
			pid: 97,
			want: &Device{
				ProductID:    97,
				RegistryName: "LIFX A19",
				LightType:    LightTypeSingleZone,
				ColorProperties: ColorProperties{
					HasColor:         true,
					TemperatureRange: TemperatureRange{Min: 1500, Max: 9000},
				},
			},
		},
		"Multizone light": {
			pid: 117,
			want: &Device{
				ProductID:    117,
				RegistryName: "LIFX Z US",
				LightType:    LightTypeMultiZone,
				ColorProperties: ColorProperties{
					HasColor:         true,
					TemperatureRange: TemperatureRange{Min: 1500, Max: 9000},
				},
			},
		},
		"Matrix light": {
			pid: 55,
			want: &Device{
				ProductID:    55,
				RegistryName: "LIFX Tile",
				LightType:    LightTypeMatrix,
				ColorProperties: ColorProperties{
					HasColor:         true,
					TemperatureRange: TemperatureRange{Min: 2500, Max: 9000},
				},
			},
		},
		"Switch": {
			pid: 89,
			want: &Device{
				ProductID:    89,
				RegistryName: "LIFX Switch",
				Type:         DeviceTypeSwitch,
			},
		},
		"Hybrid": {
			pid: 219,
			want: &Device{
				ProductID:    219,
				RegistryName: "LIFX Luna US",
				Type:         DeviceTypeHybrid,
				LightType:    LightTypeMatrix,
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
					ChainZones: [][]packets.LightHsbk{emptyZoneSlice64, emptyZoneSlice64},
				},
			},
			msg: &packets.TileStateDeviceChain{
				TileDevices:      [16]packets.TileStateDevice{{Width: 8, Height: 8}, {Width: 8, Height: 8}},
				TileDevicesCount: 2,
			},
			want: &Device{
				MatrixProperties: MatrixProperties{
					Height: 8, Width: 8, ChainLength: 2, NZones: 64, StatePackets: 1,
					ChainZones: [][]packets.LightHsbk{emptyZoneSlice64, emptyZoneSlice64},
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
					ChainZones: [][]packets.LightHsbk{emptyZoneSlice64, emptyZoneSlice64},
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
					ChainZones: [][]packets.LightHsbk{emptyZoneSlice64[:35], emptyZoneSlice64[:35]},
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
					ChainZones: [][]packets.LightHsbk{emptyZoneSlice128, emptyZoneSlice128},
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
					ChainZones: [][]packets.LightHsbk{emptyZoneSlice64},
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
					ChainZones: [][]packets.LightHsbk{emptyZoneSlice64, emptyZoneSlice64},
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
					ChainZones: [][]packets.LightHsbk{emptyZoneSlice64},
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
