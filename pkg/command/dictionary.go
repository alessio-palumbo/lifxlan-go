package command

import (
	"math/rand/v2"
	"time"
)

// colorWords returns an intentAtom setter for any supported color word.
var colorWords = map[string]func(*intentAtom){
	"white":  func(a *intentAtom) { a.setSaturation(100) },
	"red":    func(a *intentAtom) { a.setSaturation(100); a.setHue(0) },
	"orange": func(a *intentAtom) { a.setSaturation(100); a.setHue(36) },
	"yellow": func(a *intentAtom) { a.setSaturation(100); a.setHue(60) },
	"green":  func(a *intentAtom) { a.setSaturation(100); a.setHue(120) },
	"cyan":   func(a *intentAtom) { a.setSaturation(100); a.setHue(180) },
	"blue":   func(a *intentAtom) { a.setSaturation(100); a.setHue(250) },
	"purple": func(a *intentAtom) { a.setSaturation(100); a.setHue(280) },
	"pink":   func(a *intentAtom) { a.setSaturation(100); a.setHue(325) },
	"cool":   func(a *intentAtom) { a.setSaturation(100); a.setHue(325) },
	"warm":   func(a *intentAtom) { a.setSaturation(100); a.setHue(325) },
	"random": func(a *intentAtom) { a.setSaturation(100); a.setHue(float64(rand.IntN(360))) },
}

// colorWords returns an intentAtom setter for any supported property word.
var propertyWords = map[string]func(int, *intentAtom){
	"brightness": func(v int, a *intentAtom) { a.setBrightness(normalizePercent(v)) },
	"dim":        func(v int, a *intentAtom) { a.setBrightness(normalizePercent(v)) },

	"saturation": func(v int, a *intentAtom) { a.setSaturation(normalizePercent(v)) },
	"sat":        func(v int, a *intentAtom) { a.setSaturation(normalizePercent(v)) },

	"hue": func(v int, a *intentAtom) { a.setHue(normalizeHue(v)) },

	"temperature": func(v int, a *intentAtom) { a.setKelvin(normalizeKelvin(v)) },
	"kelvin":      func(v int, a *intentAtom) { a.setKelvin(normalizeKelvin(v)) },
	"k":           func(v int, a *intentAtom) { a.setKelvin(normalizeKelvin(v)) },
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

// normalizeHue returns a hue value within boundaries.
func normalizeHue(v int) float64 {
	return float64(max(0, min(v, 360)))
}

// normalizeKelvin returns a kelvin value within boundaries.
func normalizeKelvin(v int) uint16 {
	return uint16(max(1500, min(v, 9000)))
}
