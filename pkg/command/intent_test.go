package command

import (
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/stretchr/testify/assert"
)

func Test_buildintent(t *testing.T) {
	var (
		serial0 = device.Serial([8]byte{0, 0, 0, 0, 0, 0})
		serial1 = device.Serial([8]byte{0, 0, 0, 0, 0, 1})
		serial2 = device.Serial([8]byte{0, 0, 0, 0, 0, 2})
		serial3 = device.Serial([8]byte{0, 0, 0, 0, 0, 3})

		selectors = map[string][]*device.Device{
			"d00000000000": {{Serial: serial0}},
			"d00000000001": {{Serial: serial1}},
			"moon":         {{Serial: serial0}},
			"luna":         {{Serial: serial1}},
			"living room":  {{Serial: serial1}, {Serial: serial2}},
			"home":         {{Serial: serial0}, {Serial: serial1}, {Serial: serial2}, {Serial: serial3}},
		}
	)

	testCases := map[string]struct {
		atoms []intentAtom
		want  []intent
	}{
		"serial": {
			atoms: []intentAtom{
				{Kind: intentAtomSelector, Targets: selectors["moon"], NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Hue: ptr(float64(250)), Saturation: ptr(float64(100))}, PrevKind: intentAtomSelector},
			},
			want: []intent{
				{
					Targets: []*device.Device{{Serial: serial0}},
					Action:  &action{Hue: ptr(float64(250)), Saturation: ptr(float64(100))},
				},
			},
		},
		"label": {
			atoms: []intentAtom{
				{Kind: intentAtomSelector, Targets: selectors["moon"], NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Hue: ptr(float64(250)), Saturation: ptr(float64(100))}, PrevKind: intentAtomSelector},
			},
			want: []intent{
				{
					Targets: []*device.Device{{Serial: serial0}},
					Action:  &action{Hue: ptr(float64(250)), Saturation: ptr(float64(100))},
				},
			},
		},
		"multi target": {
			atoms: []intentAtom{
				{Kind: intentAtomSelector, Targets: selectors["moon"], NextKind: intentAtomSelector},
				{Kind: intentAtomSelector, Targets: selectors["living room"], PrevKind: intentAtomSelector, NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Hue: ptr(float64(120)), Saturation: ptr(float64(100))}, PrevKind: intentAtomSelector},
			},
			want: []intent{
				{
					Targets: []*device.Device{{Serial: serial0}, {Serial: serial1}, {Serial: serial2}},
					Action:  &action{Hue: ptr(float64(120)), Saturation: ptr(float64(100))},
				},
			},
		},
		"just keywords": {
			atoms: []intentAtom{
				{Kind: intentAtomSelector, Targets: selectors["home"], NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Hue: ptr(float64(120)), Saturation: ptr(float64(100))}, PrevKind: intentAtomSelector},
			},
			want: []intent{
				{
					Targets: []*device.Device{{Serial: serial0}, {Serial: serial1}, {Serial: serial2}, {Serial: serial3}},
					Action:  &action{Hue: ptr(float64(120)), Saturation: ptr(float64(100))},
				},
			},
		},
		"just keywords: flipped": {
			atoms: []intentAtom{
				{Kind: intentAtomAction, Action: &action{Power: ptr(false)}, NextKind: intentAtomSelector},
				{Kind: intentAtomSelector, Targets: selectors["home"], PrevKind: intentAtomAction},
			},
			want: []intent{
				{
					Targets: []*device.Device{{Serial: serial0}, {Serial: serial1}, {Serial: serial2}, {Serial: serial3}},
					Action:  &action{Power: ptr(false)},
				},
			},
		},
		"single action multiple targets: action last": {
			atoms: []intentAtom{
				{Kind: intentAtomSelector, Targets: selectors["moon"], NextKind: intentAtomSelector},
				{Kind: intentAtomSelector, Targets: selectors["luna"], PrevKind: intentAtomSelector, NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Hue: ptr(float64(250)), Saturation: ptr(float64(100))}, PrevKind: intentAtomSelector},
			},
			want: []intent{
				{
					Targets: []*device.Device{{Serial: serial0}, {Serial: serial1}},
					Action:  &action{Hue: ptr(float64(250)), Saturation: ptr(float64(100))},
				},
			},
		},
		"single action multiple targets: action first, consecutive targets": {
			atoms: []intentAtom{
				{Kind: intentAtomAction, Action: &action{Hue: ptr(float64(250)), Saturation: ptr(float64(100))}, NextKind: intentAtomSelector},
				{Kind: intentAtomSelector, Targets: selectors["moon"], PrevKind: intentAtomAction, NextKind: intentAtomSelector},
				{Kind: intentAtomSelector, Targets: selectors["luna"], PrevKind: intentAtomSelector},
			},
			want: []intent{
				{
					Targets: []*device.Device{{Serial: serial0}, {Serial: serial1}},
					Action:  &action{Hue: ptr(float64(250)), Saturation: ptr(float64(100))},
				},
			},
		},
		"single target, multiple actions": {
			atoms: []intentAtom{
				{Kind: intentAtomSelector, Targets: selectors["luna"], NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Hue: ptr(float64(120)), Saturation: ptr(float64(100))}, PrevKind: intentAtomSelector, NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Brightness: ptr(float64(30))}, PrevKind: intentAtomAction},
			},
			want: []intent{
				{
					Targets: []*device.Device{{Serial: serial1}},
					Action:  &action{Hue: ptr(float64(120)), Saturation: ptr(float64(100)), Brightness: ptr(float64(30))},
				},
			},
		},
		"multiple targets, multiple actions: targets first": {
			atoms: []intentAtom{
				{Kind: intentAtomSelector, Targets: selectors["luna"], NextKind: intentAtomSelector},
				{Kind: intentAtomSelector, Targets: selectors["moon"], PrevKind: intentAtomSelector, NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Hue: ptr(float64(120)), Saturation: ptr(float64(100))}, PrevKind: intentAtomSelector, NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Brightness: ptr(float64(30))}, PrevKind: intentAtomAction},
			},
			want: []intent{
				{
					Targets: []*device.Device{{Serial: serial1}, {Serial: serial0}},
					Action:  &action{Hue: ptr(float64(120)), Saturation: ptr(float64(100)), Brightness: ptr(float64(30))},
				},
			},
		},
		"multiple targets, multiple actions: actions first": {
			atoms: []intentAtom{
				{Kind: intentAtomAction, Action: &action{Hue: ptr(float64(120)), Saturation: ptr(float64(100))}, NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Brightness: ptr(float64(30))}, PrevKind: intentAtomAction, NextKind: intentAtomSelector},
				{Kind: intentAtomSelector, Targets: selectors["luna"], PrevKind: intentAtomAction, NextKind: intentAtomSelector},
				{Kind: intentAtomSelector, Targets: selectors["moon"], PrevKind: intentAtomSelector},
			},
			want: []intent{
				{
					Targets: []*device.Device{{Serial: serial1}, {Serial: serial0}},
					Action:  &action{Hue: ptr(float64(120)), Saturation: ptr(float64(100)), Brightness: ptr(float64(30))},
				},
			},
		},
		"multiple targets, different actions": {
			atoms: []intentAtom{
				{Kind: intentAtomSelector, Targets: selectors["luna"], NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Hue: ptr(float64(120)), Saturation: ptr(float64(100))}, PrevKind: intentAtomSelector, NextKind: intentAtomSelector},
				{Kind: intentAtomSelector, Targets: selectors["moon"], PrevKind: intentAtomAction, NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Brightness: ptr(float64(30))}, PrevKind: intentAtomSelector, NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Duration: ptr(5 * time.Second)}, PrevKind: intentAtomAction},
			},
			want: []intent{
				{
					Targets: []*device.Device{{Serial: serial1}},
					Action:  &action{Hue: ptr(float64(120)), Saturation: ptr(float64(100))},
				},
				{
					Targets: []*device.Device{{Serial: serial0}},
					Action:  &action{Brightness: ptr(float64(30)), Duration: ptr(5 * time.Second)},
				},
			},
		},
		"multiple property words with terminating token": {
			atoms: []intentAtom{
				{Kind: intentAtomSelector, Targets: selectors["d00000000000"], NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Saturation: ptr(float64(10))}, PrevKind: intentAtomSelector, NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Hue: ptr(float64(180))}, PrevKind: intentAtomAction, NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Kelvin: ptr(uint16(4000))}, PrevKind: intentAtomAction, NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Power: ptr(false)}, PrevKind: intentAtomAction, NextKind: intentAtomSeparator},
				{Kind: intentAtomSeparator, PrevKind: intentAtomAction, NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Power: ptr(true)}, PrevKind: intentAtomSeparator, NextKind: intentAtomSelector},
				{Kind: intentAtomSelector, Targets: selectors["luna"], PrevKind: intentAtomAction, NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Brightness: ptr(float64(10))}, PrevKind: intentAtomSelector, NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Kelvin: ptr(uint16(5000))}, PrevKind: intentAtomAction, NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Duration: ptr(500 * time.Millisecond)}, PrevKind: intentAtomAction},
			},
			want: []intent{
				{
					Targets: []*device.Device{{Serial: serial0}},
					Action:  &action{Power: ptr(false), Hue: ptr(float64(180)), Saturation: ptr(float64(10)), Kelvin: ptr(uint16(4000))},
				},
				{
					Targets: []*device.Device{{Serial: serial1}},
					Action:  &action{Power: ptr(true), Brightness: ptr(float64(10)), Kelvin: ptr(uint16(5000)), Duration: ptr(500 * time.Millisecond)},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			cmdParser := &CommandParser{selectors: selectors}
			got := cmdParser.buildIntent(tc.atoms)
			assert.Equal(t, tc.want, got)
		})
	}
}

func ptr[T any](v T) *T {
	return &v
}
