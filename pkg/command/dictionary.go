package command

import (
	"math/rand/v2"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

// colorWords returns an intentAtom setter for any supported color word.
var colorWords = map[string]func(*intentAtom){
	"white":         func(a *intentAtom) { a.setSaturation(0) },
	"candlelight":   func(a *intentAtom) { a.setSaturation(0); a.setKelvin(2200) },
	"warm white":    func(a *intentAtom) { a.setSaturation(0); a.setKelvin(2700) },
	"soft white":    func(a *intentAtom) { a.setSaturation(0); a.setKelvin(3000) },
	"neutral white": func(a *intentAtom) { a.setSaturation(0); a.setKelvin(4000) },
	"daylight":      func(a *intentAtom) { a.setSaturation(0); a.setKelvin(5600) },
	"cool white":    func(a *intentAtom) { a.setSaturation(0); a.setKelvin(6500) },
	"red":           func(a *intentAtom) { a.setSaturation(100); a.setHue(0) },
	"orange":        func(a *intentAtom) { a.setSaturation(100); a.setHue(36) },
	"yellow":        func(a *intentAtom) { a.setSaturation(100); a.setHue(60) },
	"green":         func(a *intentAtom) { a.setSaturation(100); a.setHue(120) },
	"cyan":          func(a *intentAtom) { a.setSaturation(100); a.setHue(180) },
	"blue":          func(a *intentAtom) { a.setSaturation(100); a.setHue(250) },
	"purple":        func(a *intentAtom) { a.setSaturation(100); a.setHue(280) },
	"pink":          func(a *intentAtom) { a.setSaturation(100); a.setHue(325) },
	"cool":          func(a *intentAtom) { a.setSaturation(100); a.setHue(325) },
	"warm":          func(a *intentAtom) { a.setSaturation(100); a.setHue(325) },
	"random":        func(a *intentAtom) { a.setSaturation(100); a.setHue(float64(rand.IntN(360))) },
}

var styledColorSaturations = map[string]float64{
	"pastel": 35,
	"soft":   45,
	"muted":  50,
	"washed": 25,
	"vivid":  100,
	"strong": 100,
	"deep":   90,
	"rich":   90,
}

var styledColorHues = map[string]float64{
	"red":    0,
	"orange": 36,
	"yellow": 60,
	"green":  120,
	"cyan":   180,
	"blue":   250,
	"purple": 280,
	"pink":   325,
}

func init() {
	for style, saturation := range styledColorSaturations {
		for color, hue := range styledColorHues {
			style, saturation, color, hue := style, saturation, color, hue
			colorWords[style+" "+color] = func(a *intentAtom) {
				a.setSaturation(saturation)
				a.setHue(hue)
			}
		}
	}
}

// propertyWords returns an intentAtom setter for any supported property word.
var propertyWords = map[string]func(int, *intentAtom){
	"brightness": func(v int, a *intentAtom) { a.setBrightness(normalizePercent(v)) },

	"saturation": func(v int, a *intentAtom) { a.setSaturation(normalizePercent(v)) },
	"sat":        func(v int, a *intentAtom) { a.setSaturation(normalizePercent(v)) },

	"hue": func(v int, a *intentAtom) { a.setHue(normalizeHue(v)) },

	"temperature": func(v int, a *intentAtom) { a.setKelvin(normalizeKelvin(v)) },
	"kelvin":      func(v int, a *intentAtom) { a.setKelvin(normalizeKelvin(v)) },
	"k":           func(v int, a *intentAtom) { a.setKelvin(normalizeKelvin(v)) },
}

type relativePropertyKind int

const (
	relativePropertyBrightness relativePropertyKind = iota
	relativePropertySaturation
	relativePropertyKelvin
)

type relativePropertyWord struct {
	Kind  relativePropertyKind
	Delta float64
}

// relativePropertyWords maps action words to relative property changes.
// If no explicit number is provided in the input, the configured delta is used.
var relativePropertyModifiers = map[string]float64{
	"less": -1,
	"more": 1,
}

var relativePropertyPhraseWords = map[string]relativePropertyWord{
	"bright":    {Kind: relativePropertyBrightness, Delta: 10},
	"cool":      {Kind: relativePropertyKelvin, Delta: -500},
	"deep":      {Kind: relativePropertySaturation, Delta: 10},
	"intense":   {Kind: relativePropertySaturation, Delta: 10},
	"muted":     {Kind: relativePropertySaturation, Delta: -10},
	"pastel":    {Kind: relativePropertySaturation, Delta: -10},
	"saturated": {Kind: relativePropertySaturation, Delta: 10},
	"soft":      {Kind: relativePropertySaturation, Delta: -10},
	"strong":    {Kind: relativePropertySaturation, Delta: 10},
	"vivid":     {Kind: relativePropertySaturation, Delta: 10},
	"warm":      {Kind: relativePropertyKelvin, Delta: 500},
}

var relativePropertyWords = map[string]relativePropertyWord{
	"dim":      {Kind: relativePropertyBrightness, Delta: -10},
	"darken":   {Kind: relativePropertyBrightness, Delta: -10},
	"darker":   {Kind: relativePropertyBrightness, Delta: -10},
	"lower":    {Kind: relativePropertyBrightness, Delta: -10},
	"brighten": {Kind: relativePropertyBrightness, Delta: 10},
	"brighter": {Kind: relativePropertyBrightness, Delta: 10},
	"raise":    {Kind: relativePropertyBrightness, Delta: 10},

	"muted":  {Kind: relativePropertySaturation, Delta: -10},
	"soften": {Kind: relativePropertySaturation, Delta: -10},
	"softer": {Kind: relativePropertySaturation, Delta: -10},
	"washed": {Kind: relativePropertySaturation, Delta: -10},
	"deeper": {Kind: relativePropertySaturation, Delta: 10},
	"rich":   {Kind: relativePropertySaturation, Delta: 10},
	"richer": {Kind: relativePropertySaturation, Delta: 10},
	"strong": {Kind: relativePropertySaturation, Delta: 10},
	"vivid":  {Kind: relativePropertySaturation, Delta: 10},

	"cool":   {Kind: relativePropertyKelvin, Delta: -500},
	"cooler": {Kind: relativePropertyKelvin, Delta: -500},
	"warm":   {Kind: relativePropertyKelvin, Delta: 500},
	"warmer": {Kind: relativePropertyKelvin, Delta: 500},
}

// durationWords returns an intentAtom setter for any supported duration word.
var durationWords = map[string]func(int, *intentAtom){
	"ms":           func(v int, a *intentAtom) { a.setDuration(time.Duration(v) * time.Millisecond) },
	"millisecond":  func(v int, a *intentAtom) { a.setDuration(time.Duration(v) * time.Millisecond) },
	"milliseconds": func(v int, a *intentAtom) { a.setDuration(time.Duration(v) * time.Millisecond) },

	"s":       func(v int, a *intentAtom) { a.setDuration(time.Duration(v) * time.Second) },
	"sec":     func(v int, a *intentAtom) { a.setDuration(time.Duration(v) * time.Second) },
	"secs":    func(v int, a *intentAtom) { a.setDuration(time.Duration(v) * time.Second) },
	"seconds": func(v int, a *intentAtom) { a.setDuration(time.Duration(v) * time.Second) },

	"m":       func(v int, a *intentAtom) { a.setDuration(time.Duration(v) * time.Second) },
	"min":     func(v int, a *intentAtom) { a.setDuration(time.Duration(v) * time.Second) },
	"mins":    func(v int, a *intentAtom) { a.setDuration(time.Duration(v) * time.Second) },
	"minute":  func(v int, a *intentAtom) { a.setDuration(time.Duration(v) * time.Second) },
	"minutes": func(v int, a *intentAtom) { a.setDuration(time.Duration(v) * time.Second) },
}

// normalizePercent returns a normalized percent value.
func normalizePercent(v int) float64 {
	return float64(max(0, min(v, 100)))
}

func normalizeRelativeBrightness(v float64) float64 {
	return max(1, min(v, 100))
}

func normalizeRelativePercent(v float64) float64 {
	return max(0, min(v, 100))
}

func normalizeRelativeKelvin(v int, r device.TemperatureRange) uint16 {
	minKelvin, maxKelvin := r.Min, r.Max
	if minKelvin == 0 || maxKelvin == 0 {
		minKelvin, maxKelvin = 1500, 9000
	}
	return uint16(max(minKelvin, min(v, maxKelvin)))
}

// normalizeHue returns a hue value within boundaries.
func normalizeHue(v int) float64 {
	return float64(max(0, min(v, 360)))
}

// normalizeKelvin returns a kelvin value within boundaries.
func normalizeKelvin(v int) uint16 {
	return uint16(max(1500, min(v, 9000)))
}
