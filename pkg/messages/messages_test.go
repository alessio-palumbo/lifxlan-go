package messages

import (
	"math"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/enums"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	"github.com/stretchr/testify/assert"
)

func TestSetPowerOn(t *testing.T) {
	testCases := map[string]struct {
		d    []time.Duration
		want *protocol.Message
	}{
		"no duration": {
			want: protocol.NewMessage(&packets.DeviceSetPower{Level: math.MaxUint16}),
		},
		"with duration < default": {
			d:    []time.Duration{time.Millisecond},
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
		"with duration < default": {
			d:    []time.Duration{time.Millisecond},
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
				Period: uint32(defaultPeriod.Milliseconds()), SetHue: true,
				Color: packets.LightHsbk{Hue: 32768},
			}),
		},
		"saturation only": {
			s: ptr(float64(100)),
			want: protocol.NewMessage(&packets.LightSetWaveformOptional{
				Waveform: enums.LightWaveformLIGHTWAVEFORMSAW, Cycles: 1.0,
				Period: uint32(defaultPeriod.Milliseconds()), SetSaturation: true,
				Color: packets.LightHsbk{Saturation: math.MaxUint16},
			}),
		},
		"brightness only": {
			b: ptr(float64(50)),
			want: protocol.NewMessage(&packets.LightSetWaveformOptional{
				Waveform: enums.LightWaveformLIGHTWAVEFORMSAW, Cycles: 1.0,
				Period: uint32(defaultPeriod.Milliseconds()), SetBrightness: true,
				Color: packets.LightHsbk{Brightness: 32768},
			}),
		},
		"kelvin only": {
			k: ptr(uint16(5000)),
			want: protocol.NewMessage(&packets.LightSetWaveformOptional{
				Waveform: enums.LightWaveformLIGHTWAVEFORMSAW, Cycles: 1.0,
				Period: uint32(defaultPeriod.Milliseconds()), SetKelvin: true,
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

func ptr[T any](v T) *T {
	return &v
}
