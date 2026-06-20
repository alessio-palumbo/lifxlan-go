package effects

import (
	"reflect"
	"testing"
)

func TestRandomIsDeterministicForSeed(t *testing.T) {
	first := randomSequence(NewRandom(42), 5)
	second := randomSequence(NewRandom(42), 5)

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("sequences differ for same seed:\nfirst=%#v\nsecond=%#v", first, second)
	}
}

func TestRandomDiffersForDifferentSeeds(t *testing.T) {
	first := randomSequence(NewRandom(42), 5)
	second := randomSequence(NewRandom(43), 5)

	if reflect.DeepEqual(first, second) {
		t.Fatalf("sequences matched for different seeds: %#v", first)
	}
}

func TestRandomResetRewindsSequence(t *testing.T) {
	r := NewRandom(42)
	first := randomSequence(r, 5)

	r.Reset()
	second := randomSequence(r, 5)

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("second sequence differed after reset:\nfirst=%#v\nsecond=%#v", first, second)
	}
}

func TestRandomSafeFallbacks(t *testing.T) {
	var r *Random
	if got := r.Seed(); got != 0 {
		t.Fatalf("nil Seed = %d, want 0", got)
	}
	if got := r.Float64(); got != 0 {
		t.Fatalf("nil Float64 = %f, want 0", got)
	}
	if got := r.IntN(10); got != 0 {
		t.Fatalf("nil IntN = %d, want 0", got)
	}
	if got := NewRandom(1).IntN(0); got != 0 {
		t.Fatalf("IntN(0) = %d, want 0", got)
	}
	if got := NewRandom(1).Color(nil); got != DefaultColor {
		t.Fatalf("Color(nil) = %#v, want DefaultColor", got)
	}
}

func TestRandomColorIsDeterministic(t *testing.T) {
	colors := []Color{color(10), color(20), color(30)}

	first := NewRandom(7)
	second := NewRandom(7)

	for range 10 {
		if got, want := first.Color(colors), second.Color(colors); got != want {
			t.Fatalf("Color sequence differed: got %#v want %#v", got, want)
		}
	}
}

func TestRandomPaletteColorUsesPaletteColors(t *testing.T) {
	r := NewRandom(7)
	palette := Palette{
		Base:        []Color{color(10)},
		Accents:     []Color{color(20)},
		Backgrounds: []Color{color(30)},
	}

	got := r.PaletteColor(palette)
	if got != color(10) && got != color(20) && got != color(30) {
		t.Fatalf("PaletteColor = %#v, want one palette color", got)
	}

	if got := r.PaletteColor(Palette{}); got != DefaultColor {
		t.Fatalf("PaletteColor(empty) = %#v, want DefaultColor", got)
	}
}

func randomSequence(r *Random, count int) []int {
	values := make([]int, count)
	for i := range values {
		values[i] = r.IntN(1000)
	}
	return values
}
