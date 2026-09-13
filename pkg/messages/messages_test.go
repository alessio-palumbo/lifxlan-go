package messages

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/enums"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetPowerOn(t *testing.T) {
	testCases := map[string]struct {
		d    []time.Duration
		want *protocol.Message
	}{
		"no duration": {
			want: protocol.NewMessage(&packets.DeviceSetPower{Level: math.MaxUint16}),
		},
		"with duration": {
			d:    []time.Duration{5 * time.Second},
			want: protocol.NewMessage(&packets.LightSetPower{Level: math.MaxUint16, Duration: 5000}),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := SetPowerOn(tc.d...)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestSetPowerOff(t *testing.T) {
	testCases := map[string]struct {
		d    []time.Duration
		want *protocol.Message
	}{
		"no duration": {
			want: protocol.NewMessage(&packets.DeviceSetPower{Level: 0}),
		},
		"with duration": {
			d:    []time.Duration{5 * time.Second},
			want: protocol.NewMessage(&packets.LightSetPower{Level: 0, Duration: 5000}),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := SetPowerOff(tc.d...)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestSetColor(t *testing.T) {
	testCases := map[string]struct {
		h, s, b *float64
		k       *uint16
		d       time.Duration
		w       enums.LightWaveform
		want    *protocol.Message
	}{
		"hue only": {
			h: ptr(float64(180)),
			want: protocol.NewMessage(&packets.LightSetWaveformOptional{
				Waveform: enums.LightWaveformLIGHTWAVEFORMSAW, Cycles: 1.0,
				Period: 0, SetHue: true,
				Color: packets.LightHsbk{Hue: 32768},
			}),
		},
		"saturation only": {
			s: ptr(float64(100)),
			want: protocol.NewMessage(&packets.LightSetWaveformOptional{
				Waveform: enums.LightWaveformLIGHTWAVEFORMSAW, Cycles: 1.0,
				Period: 0, SetSaturation: true,
				Color: packets.LightHsbk{Saturation: math.MaxUint16},
			}),
		},
		"brightness only": {
			b: ptr(float64(50)),
			want: protocol.NewMessage(&packets.LightSetWaveformOptional{
				Waveform: enums.LightWaveformLIGHTWAVEFORMSAW, Cycles: 1.0,
				Period: 0, SetBrightness: true,
				Color: packets.LightHsbk{Brightness: 32768},
			}),
		},
		"kelvin only": {
			k: ptr(uint16(5000)),
			want: protocol.NewMessage(&packets.LightSetWaveformOptional{
				Waveform: enums.LightWaveformLIGHTWAVEFORMSAW, Cycles: 1.0,
				Period: 0, SetKelvin: true,
				Color: packets.LightHsbk{Kelvin: 5000},
			}),
		},
		"all fields": {
			h: ptr(float64(180)),
			s: ptr(float64(100)),
			b: ptr(float64(50)),
			k: ptr(uint16(5000)),
			d: 5 * time.Second,
			want: protocol.NewMessage(&packets.LightSetWaveformOptional{
				Waveform: enums.LightWaveformLIGHTWAVEFORMSAW, Cycles: 1.0,
				Period: uint32(5000),
				SetHue: true, SetSaturation: true, SetBrightness: true, SetKelvin: true,
				Color: packets.LightHsbk{Hue: 32768, Saturation: math.MaxUint16, Brightness: 32768, Kelvin: 5000},
			}),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := SetColor(tc.h, tc.s, tc.b, tc.k, tc.d, tc.w)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestSetLabel(t *testing.T) {
	for name, label := range map[string]string{
		"label":              "Desk",
		"empty label":        "",
		"32 ASCII bytes":     strings.Repeat("x", 32),
		"32 multibyte bytes": strings.Repeat("é", 16),
	} {
		t.Run(name, func(t *testing.T) {
			var wantLabel [32]byte
			copy(wantLabel[:], label)
			got, err := SetLabel(label)
			require.NoError(t, err)
			want := protocol.NewMessage(&packets.DeviceSetLabel{Label: wantLabel})
			want.SetResponseRequired(true)
			assert.Equal(t, want, got)
			assertResponseRequired(t, got)
		})
	}
}

func TestSetLocation(t *testing.T) {
	id := device.LocationID{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0x4d, 0xef, 0x8e, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10}
	updatedAt := time.Unix(123, 456)

	got, err := SetLocation(id, "Home", updatedAt)
	require.NoError(t, err)
	want := protocol.NewMessage(&packets.DeviceSetLocation{
		Location:  [16]byte(id),
		Label:     [32]byte{'H', 'o', 'm', 'e'},
		UpdatedAt: 123000000456,
	})
	want.SetResponseRequired(true)
	assert.Equal(t, want, got)
	assertResponseRequired(t, got)
}

func TestSetGroup(t *testing.T) {
	id := device.GroupID{0xff, 0xee, 0xdd, 0xcc, 0xbb, 0xaa, 0x49, 0x88, 0xb7, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11, 0x00}
	updatedAt := time.Unix(789, 123)

	got, err := SetGroup(id, "Office", updatedAt)
	require.NoError(t, err)
	want := protocol.NewMessage(&packets.DeviceSetGroup{
		Group:     [16]byte(id),
		Label:     [32]byte{'O', 'f', 'f', 'i', 'c', 'e'},
		UpdatedAt: 789000000123,
	})
	want.SetResponseRequired(true)
	assert.Equal(t, want, got)
	assertResponseRequired(t, got)
}

func TestSetMetadataRejectsInvalidLabels(t *testing.T) {
	for name, label := range map[string]string{
		"more than 32 bytes": "123456789012345678901234567890123",
		"multibyte overflow": "ééééééééééééééééé",
		"invalid UTF-8":      string([]byte{0xff}),
		"null byte":          "Home\x00Office",
	} {
		t.Run(name, func(t *testing.T) {
			_, labelErr := SetLabel(label)
			_, locationErr := SetLocation(device.LocationID{}, label, time.Unix(0, 0))
			_, groupErr := SetGroup(device.GroupID{}, label, time.Unix(0, 0))
			assert.Error(t, labelErr)
			assert.Error(t, locationErr)
			assert.Error(t, groupErr)
		})
	}
}

func TestSetLocationAndGroupRejectPreEpochUpdatedAt(t *testing.T) {
	for name, updatedAt := range map[string]time.Time{
		"before epoch":     time.Unix(-1, 0),
		"after uint64 max": time.Date(3000, time.January, 1, 0, 0, 0, 0, time.UTC),
	} {
		t.Run(name, func(t *testing.T) {
			_, locationErr := SetLocation(device.LocationID{}, "Home", updatedAt)
			_, groupErr := SetGroup(device.GroupID{}, "Office", updatedAt)
			assert.Error(t, locationErr)
			assert.Error(t, groupErr)
		})
	}
}

func TestFirmwareEffectMessages(t *testing.T) {
	assert.Equal(t, protocol.NewMessage(&packets.MultiZoneGetEffect{}), GetMultizoneEffect())
	assert.Equal(t, protocol.NewMessage(&packets.TileGetEffect{}), GetMatrixEffect())

	speed := time.Second
	for name, msg := range map[string]*protocol.Message{
		"multizone off":  SetMultizoneEffectOff(),
		"multizone move": SetMultizoneMoveEffect(speed, true),
		"matrix off":     SetMatrixEffectOff(),
		"matrix flame":   SetMatrixFlameEffect(speed),
		"matrix morph":   SetMatrixMorphEffect(speed),
		"matrix clouds":  SetMatrixCloudsEffect(speed, nil),
		"matrix sunrise": SetMatrixSunriseEffect(&speed),
		"matrix sunset":  SetMatrixSunsetEffect(&speed, true),
	} {
		t.Run(name, func(t *testing.T) {
			assertResponseRequired(t, msg)
		})
	}
}

func ptr[T any](v T) *T {
	return &v
}

func assertResponseRequired(t *testing.T, msg *protocol.Message) {
	t.Helper()
	encoded, err := msg.MarshalBinary()
	require.NoError(t, err)
	assert.NotZero(t, encoded[22]&1, "response-required flag is not set")
}

func TestRelayMessages(t *testing.T) {
	assert.Equal(t, protocol.NewMessage(&packets.RelayGetPower{RelayIndex: 2}), GetRelayPower(2))
	assert.Equal(t, protocol.NewMessage(&packets.RelaySetPower{RelayIndex: 2, Level: math.MaxUint16}), SetRelayPower(2, true))
	assert.Equal(t, protocol.NewMessage(&packets.RelaySetPower{RelayIndex: 2, Level: 0}), SetRelayPower(2, false))
	assert.Equal(t, protocol.NewMessage(&packets.RelaySetPower{RelayIndex: 2, Level: 42}), SetRelayPowerLevel(2, 42))
}

func TestButtonConfigMessages(t *testing.T) {
	on := device.Color{Hue: 180, Saturation: 100, Brightness: 50, Kelvin: 3500}
	off := device.Color{Brightness: 10, Kelvin: 2700}

	assert.Equal(t, protocol.NewMessage(&packets.ButtonGetConfig{}), GetButtonConfig())
	assert.Equal(t, protocol.NewMessage(&packets.ButtonSetConfig{
		HapticDurationMs: 250,
		BacklightOnColor: packets.ButtonBacklightHsbk{
			Hue:        32768,
			Saturation: math.MaxUint16,
			Brightness: 32768,
			Kelvin:     3500,
		},
		BacklightOffColor: packets.ButtonBacklightHsbk{
			Brightness: 6554,
			Kelvin:     2700,
		},
	}), SetButtonConfig(250, on, off))
}
