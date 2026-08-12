package command

import (
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/stretchr/testify/assert"
)

func Test_buildIntentAtoms(t *testing.T) {
	var (
		serial0 = device.Serial([8]byte{0, 0, 0, 0, 0, 0})
		serial1 = device.Serial([8]byte{0, 0, 0, 0, 0, 1})
		serial2 = device.Serial([8]byte{0, 0, 0, 0, 0, 2})
		serial3 = device.Serial([8]byte{0, 0, 0, 0, 0, 3})

		selectors = map[string][]*device.Device{
			"000000000000": {{Serial: serial0}},
			"000000000001": {{Serial: serial1}},
			"moon":         {{Serial: serial0}},
			"luna":         {{Serial: serial1}},
			"living room":  {{Serial: serial1}, {Serial: serial2}},
			"home":         {{Serial: serial0}, {Serial: serial1}, {Serial: serial2}, {Serial: serial3}},
		}
	)

	testCases := map[string]struct {
		tokens []token
		want   []intentAtom
	}{
		"serial": {
			tokens: []token{
				{Raw: "000000000000", Kind: tokenSelector},
				{Raw: "blue", Kind: tokenColor},
			},
			want: []intentAtom{
				{Kind: intentAtomSelector, Targets: selectors["moon"], NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Hue: ptr(float64(250)), Saturation: ptr(float64(100))}, PrevKind: intentAtomSelector},
			},
		},
		"label": {
			tokens: []token{
				{Raw: "moon", Kind: tokenSelector},
				{Raw: "blue", Kind: tokenColor},
			},
			want: []intentAtom{
				{Kind: intentAtomSelector, Targets: selectors["moon"], NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Hue: ptr(float64(250)), Saturation: ptr(float64(100))}, PrevKind: intentAtomSelector},
			},
		},
		"multi target": {
			tokens: []token{
				{Raw: "moon", Kind: tokenSelector},
				{Raw: "living room", Kind: tokenSelector},
				{Raw: "green", Kind: tokenColor},
			},
			want: []intentAtom{
				{Kind: intentAtomSelector, Targets: selectors["moon"], NextKind: intentAtomSelector},
				{Kind: intentAtomSelector, Targets: selectors["living room"], PrevKind: intentAtomSelector, NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Hue: ptr(float64(120)), Saturation: ptr(float64(100))}, PrevKind: intentAtomSelector},
			},
		},
		"just keywords": {
			tokens: []token{
				{Raw: "home", Kind: tokenSelector},
				{Raw: "green", Kind: tokenColor},
			},
			want: []intentAtom{
				{Kind: intentAtomSelector, Targets: selectors["home"], NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Hue: ptr(float64(120)), Saturation: ptr(float64(100))}, PrevKind: intentAtomSelector},
			},
		},
		"just keywords: flipped": {
			tokens: []token{
				{Raw: "off", Kind: tokenPower},
				{Raw: "home", Kind: tokenSelector},
			},
			want: []intentAtom{
				{Kind: intentAtomAction, Action: &action{Power: ptr(false)}, NextKind: intentAtomSelector},
				{Kind: intentAtomSelector, Targets: selectors["home"], PrevKind: intentAtomAction},
			},
		},
		"single action multiple targets: action last": {
			tokens: []token{
				{Raw: "moon", Kind: tokenSelector},
				{Raw: "luna", Kind: tokenSelector},
				{Raw: "blue", Kind: tokenColor},
			},
			want: []intentAtom{
				{Kind: intentAtomSelector, Targets: selectors["moon"], NextKind: intentAtomSelector},
				{Kind: intentAtomSelector, Targets: selectors["luna"], PrevKind: intentAtomSelector, NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Hue: ptr(float64(250)), Saturation: ptr(float64(100))}, PrevKind: intentAtomSelector},
			},
		},
		"single action multiple targets: action first, consecutive targets": {
			tokens: []token{
				{Raw: "blue", Kind: tokenColor},
				{Raw: "moon", Kind: tokenSelector},
				{Raw: "luna", Kind: tokenSelector},
			},
			want: []intentAtom{
				{Kind: intentAtomAction, Action: &action{Hue: ptr(float64(250)), Saturation: ptr(float64(100))}, NextKind: intentAtomSelector},
				{Kind: intentAtomSelector, Targets: selectors["moon"], PrevKind: intentAtomAction, NextKind: intentAtomSelector},
				{Kind: intentAtomSelector, Targets: selectors["luna"], PrevKind: intentAtomSelector},
			},
		},
		"relative brightness before selector with explicit amount": {
			tokens: []token{
				{Raw: "dim", Kind: tokenRelativeProperty},
				{Raw: "luna", Kind: tokenSelector},
				{Raw: "30%", Kind: tokenNumber, Value: 30},
			},
			want: []intentAtom{
				{Kind: intentAtomAction, Action: &action{BrightnessDelta: ptr(float64(-30))}, NextKind: intentAtomSelector},
				{Kind: intentAtomSelector, Targets: selectors["luna"], PrevKind: intentAtomAction},
			},
		},
		"relative brightness after selector with default amount": {
			tokens: []token{
				{Raw: "luna", Kind: tokenSelector},
				{Raw: "brighter", Kind: tokenRelativeProperty},
			},
			want: []intentAtom{
				{Kind: intentAtomSelector, Targets: selectors["luna"], NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{BrightnessDelta: ptr(float64(10))}, PrevKind: intentAtomSelector},
			},
		},
		"relative brightness with amount before selector": {
			tokens: []token{
				{Raw: "brighten", Kind: tokenRelativeProperty},
				{Raw: "20%", Kind: tokenNumber, Value: 20},
				{Raw: "luna", Kind: tokenSelector},
			},
			want: []intentAtom{
				{Kind: intentAtomAction, Action: &action{BrightnessDelta: ptr(float64(20))}, NextKind: intentAtomSelector},
				{Kind: intentAtomSelector, Targets: selectors["luna"], PrevKind: intentAtomAction},
			},
		},
		"relative saturation phrase after selector": {
			tokens: []token{
				{Raw: "luna", Kind: tokenSelector},
				{Raw: "more vivid", Kind: tokenRelativeProperty, Value: 1, Suffix: "vivid"},
			},
			want: []intentAtom{
				{Kind: intentAtomSelector, Targets: selectors["luna"], NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{SaturationDelta: ptr(float64(10))}, PrevKind: intentAtomSelector},
			},
		},
		"relative saturation phrase before selector with explicit amount": {
			tokens: []token{
				{Raw: "less intense", Kind: tokenRelativeProperty, Value: -1, Suffix: "intense"},
				{Raw: "luna", Kind: tokenSelector},
				{Raw: "20%", Kind: tokenNumber, Value: 20},
			},
			want: []intentAtom{
				{Kind: intentAtomAction, Action: &action{SaturationDelta: ptr(float64(-20))}, NextKind: intentAtomSelector},
				{Kind: intentAtomSelector, Targets: selectors["luna"], PrevKind: intentAtomAction},
			},
		},
		"relative brightness phrase": {
			tokens: []token{
				{Raw: "luna", Kind: tokenSelector},
				{Raw: "less bright", Kind: tokenRelativeProperty, Value: -1, Suffix: "bright"},
			},
			want: []intentAtom{
				{Kind: intentAtomSelector, Targets: selectors["luna"], NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{BrightnessDelta: ptr(float64(-10))}, PrevKind: intentAtomSelector},
			},
		},
		"relative temperature phrase reverses base meaning": {
			tokens: []token{
				{Raw: "less cool", Kind: tokenRelativeProperty, Value: -1, Suffix: "cool"},
				{Raw: "luna", Kind: tokenSelector},
			},
			want: []intentAtom{
				{Kind: intentAtomAction, Action: &action{KelvinDelta: ptr(500)}, NextKind: intentAtomSelector},
				{Kind: intentAtomSelector, Targets: selectors["luna"], PrevKind: intentAtomAction},
			},
		},
		"white temperature phrase": {
			tokens: []token{
				{Raw: "luna", Kind: tokenSelector},
				{Raw: "warm white", Kind: tokenColor},
			},
			want: []intentAtom{
				{Kind: intentAtomSelector, Targets: selectors["luna"], NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Saturation: ptr(float64(0)), Kelvin: ptr(uint16(2700))}, PrevKind: intentAtomSelector},
			},
		},
		"styled color phrase": {
			tokens: []token{
				{Raw: "luna", Kind: tokenSelector},
				{Raw: "pastel blue", Kind: tokenColor},
			},
			want: []intentAtom{
				{Kind: intentAtomSelector, Targets: selectors["luna"], NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Hue: ptr(float64(250)), Saturation: ptr(float64(35))}, PrevKind: intentAtomSelector},
			},
		},
		"single target, multiple actions": {
			tokens: []token{
				{Raw: "luna", Kind: tokenSelector},
				{Raw: "green", Kind: tokenColor},
				{Raw: "brightness", Kind: tokenProperty},
				{Raw: "30%", Kind: tokenNumber, Value: 30},
			},
			want: []intentAtom{
				{Kind: intentAtomSelector, Targets: selectors["luna"], NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Hue: ptr(float64(120)), Saturation: ptr(float64(100))}, PrevKind: intentAtomSelector, NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Brightness: ptr(float64(30))}, PrevKind: intentAtomAction},
			},
		},
		"multiple targets, multiple actions: targets first": {
			tokens: []token{
				{Raw: "luna", Kind: tokenSelector},
				{Raw: "moon", Kind: tokenSelector},
				{Raw: "green", Kind: tokenColor},
				{Raw: "brightness", Kind: tokenProperty},
				{Raw: "30%", Kind: tokenNumber, Value: 30},
			},
			want: []intentAtom{
				{Kind: intentAtomSelector, Targets: selectors["luna"], NextKind: intentAtomSelector},
				{Kind: intentAtomSelector, Targets: selectors["moon"], PrevKind: intentAtomSelector, NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Hue: ptr(float64(120)), Saturation: ptr(float64(100))}, PrevKind: intentAtomSelector, NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Brightness: ptr(float64(30))}, PrevKind: intentAtomAction},
			},
		},
		"multiple targets, multiple actions: actions first": {
			tokens: []token{
				{Raw: "green", Kind: tokenColor},
				{Raw: "brightness", Kind: tokenProperty},
				{Raw: "30%", Kind: tokenNumber, Value: 30},
				{Raw: "luna", Kind: tokenSelector},
				{Raw: "moon", Kind: tokenSelector},
			},
			want: []intentAtom{
				{Kind: intentAtomAction, Action: &action{Hue: ptr(float64(120)), Saturation: ptr(float64(100))}, NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Brightness: ptr(float64(30))}, PrevKind: intentAtomAction, NextKind: intentAtomSelector},
				{Kind: intentAtomSelector, Targets: selectors["luna"], PrevKind: intentAtomAction, NextKind: intentAtomSelector},
				{Kind: intentAtomSelector, Targets: selectors["moon"], PrevKind: intentAtomSelector},
			},
		},
		"multiple targets, different actions": {
			tokens: []token{
				{Raw: "luna", Kind: tokenSelector},
				{Raw: "green", Kind: tokenColor},
				{Raw: "moon", Kind: tokenSelector},
				{Raw: "brightness", Kind: tokenProperty},
				{Raw: "30%", Kind: tokenNumber, Value: 30},
				{Raw: "5s", Kind: tokenNumberD, Value: 5, Suffix: "s"},
			},
			want: []intentAtom{
				{Kind: intentAtomSelector, Targets: selectors["luna"], NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Hue: ptr(float64(120)), Saturation: ptr(float64(100))}, PrevKind: intentAtomSelector, NextKind: intentAtomSelector},
				{Kind: intentAtomSelector, Targets: selectors["moon"], PrevKind: intentAtomAction, NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Brightness: ptr(float64(30))}, PrevKind: intentAtomSelector, NextKind: intentAtomAction},
				{Kind: intentAtomAction, Action: &action{Duration: ptr(5 * time.Second)}, PrevKind: intentAtomAction},
			},
		},
		"minute duration suffix": {
			tokens: []token{
				{Raw: "5m", Kind: tokenNumberD, Value: 5, Suffix: "m"},
			},
			want: []intentAtom{
				{Kind: intentAtomAction, Action: &action{Duration: ptr(5 * time.Minute)}},
			},
		},
		"minute duration word": {
			tokens: []token{
				{Raw: "2", Kind: tokenNumber, Value: 2},
				{Raw: "minutes", Kind: tokenDuration},
			},
			want: []intentAtom{
				{Kind: intentAtomAction, Action: &action{Duration: ptr(2 * time.Minute)}},
			},
		},
		"multiple property words with terminating token": {
			tokens: []token{
				{Raw: "000000000000", Kind: tokenSelector},
				{Raw: "10%", Kind: tokenNumber, Value: 10},
				{Raw: "sat", Kind: tokenProperty},
				{Raw: "180", Kind: tokenNumber, Value: 180},
				{Raw: "hue", Kind: tokenProperty},
				{Raw: "4000k", Kind: tokenNumberK, Value: 4000, Suffix: "k"},
				{Raw: "off", Kind: tokenPower},
				{Raw: "next", Kind: tokenSeparator},
				{Raw: "on", Kind: tokenPower},
				{Raw: "luna", Kind: tokenSelector},
				{Raw: "10%", Kind: tokenNumber, Value: 10},
				{Raw: "brightness", Kind: tokenProperty},
				{Raw: "5000", Kind: tokenNumber, Value: 5000},
				{Raw: "kelvin", Kind: tokenProperty},
				{Raw: "500", Kind: tokenNumber, Value: 500},
				{Raw: "ms", Kind: tokenDuration},
			},
			want: []intentAtom{
				{Kind: intentAtomSelector, Targets: selectors["000000000000"], NextKind: intentAtomAction},
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
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			cmdParser := &CommandParser{selectors: selectors}
			got := cmdParser.buildIntentAtoms(tc.tokens)
			assert.Equal(t, tc.want, got)
		})
	}
}
