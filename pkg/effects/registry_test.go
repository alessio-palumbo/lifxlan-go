package effects

import (
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

func TestDefinitionsDeterministicAndIncludeBuiltins(t *testing.T) {
	first := Definitions()
	second := Definitions()
	if !reflect.DeepEqual(definitionIDs(first), definitionIDs(second)) {
		t.Fatalf("definition order changed between calls:\nfirst=%#v\nsecond=%#v", definitionIDs(first), definitionIDs(second))
	}

	ids := make([]EffectID, len(first))
	for i, def := range first {
		ids[i] = def.ID
	}
	if !slices.IsSortedFunc(ids, func(a, b EffectID) int {
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	}) {
		t.Fatalf("definitions are not sorted by ID: %#v", ids)
	}

	for _, id := range []EffectID{EffectSolid, EffectGradient, EffectSweep} {
		if !slices.Contains(ids, id) {
			t.Fatalf("missing built-in definition %q from %#v", id, ids)
		}
	}
}

func definitionIDs(defs []EffectDefinition) []EffectID {
	ids := make([]EffectID, len(defs))
	for i, def := range defs {
		ids[i] = def.ID
	}
	return ids
}

func TestNewConstructsBuiltInEffects(t *testing.T) {
	tests := map[string]struct {
		config Config
		want   any
	}{
		"solid": {
			config: Config{ID: EffectSolid, Params: map[string]any{
				"color": color(10),
			}},
			want: &Solid{},
		},
		"gradient": {
			config: Config{ID: EffectGradient, Params: map[string]any{
				"palette": Palette{Base: []Color{color(10)}},
			}},
			want: &Gradient{},
		},
		"sweep": {
			config: Config{ID: EffectSweep, Params: map[string]any{
				"palette": Palette{Accents: []Color{color(90)}, Backgrounds: []Color{color(200)}},
			}},
			want: &Sweep{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			effect, err := New(tt.config, Capabilities{LightType: device.LightTypeSingleZone})
			if err != nil {
				t.Fatal(err)
			}
			if reflect.TypeOf(effect) != reflect.TypeOf(tt.want) {
				t.Fatalf("effect = %T, want %T", effect, tt.want)
			}
		})
	}
}

func TestNewRejectsUnknownEffect(t *testing.T) {
	_, err := New(Config{ID: "missing"}, Capabilities{LightType: device.LightTypeSingleZone})
	if !errors.Is(err, ErrUnknownEffect) {
		t.Fatalf("error = %v, want %v", err, ErrUnknownEffect)
	}
}

func TestRegisterRejectsDuplicateID(t *testing.T) {
	def, ok := Definition(EffectSolid)
	if !ok {
		t.Fatal("missing solid definition")
	}

	if err := Register(def); !errors.Is(err, ErrDuplicateEffect) {
		t.Fatalf("error = %v, want %v", err, ErrDuplicateEffect)
	}
}

func TestDefinitionsReturnCopies(t *testing.T) {
	def, ok := Definition(EffectGradient)
	if !ok {
		t.Fatal("missing gradient definition")
	}
	def.DeviceKinds[0] = device.LightType(99)
	def.Params[0].Key = "mutated"
	defaultPalette := def.Params[0].Default.(Palette)
	defaultPalette.Base[0] = color(99)

	next, ok := Definition(EffectGradient)
	if !ok {
		t.Fatal("missing gradient definition after mutation")
	}
	if next.DeviceKinds[0] == device.LightType(99) {
		t.Fatal("Definition returned mutable DeviceKinds backing slice")
	}
	if next.Params[0].Key == "mutated" {
		t.Fatal("Definition returned mutable Params backing slice")
	}
	nextPalette := next.Params[0].Default.(Palette)
	if nextPalette.Base[0] == color(99) {
		t.Fatal("Definition returned mutable default palette backing slice")
	}

	defs := Definitions()
	gradientIndex := slices.IndexFunc(defs, func(def EffectDefinition) bool {
		return def.ID == EffectGradient
	})
	if gradientIndex < 0 {
		t.Fatal("missing gradient definition from Definitions")
	}
	defs[gradientIndex].DeviceKinds[0] = device.LightType(99)
	defs[gradientIndex].Params[0].Key = "mutated"
	defsPalette := defs[gradientIndex].Params[0].Default.(Palette)
	defsPalette.Base[0] = color(99)

	fresh := Definitions()
	gradientIndex = slices.IndexFunc(fresh, func(def EffectDefinition) bool {
		return def.ID == EffectGradient
	})
	if fresh[gradientIndex].DeviceKinds[0] == device.LightType(99) {
		t.Fatal("Definitions returned mutable DeviceKinds backing slice")
	}
	if fresh[gradientIndex].Params[0].Key == "mutated" {
		t.Fatal("Definitions returned mutable Params backing slice")
	}
	freshPalette := fresh[gradientIndex].Params[0].Default.(Palette)
	if freshPalette.Base[0] == color(99) {
		t.Fatal("Definitions returned mutable default palette backing slice")
	}
}

func TestValidateParamsClonesDefaults(t *testing.T) {
	defs := []ParamDefinition{{
		Key:  "palette",
		Kind: ParamPalette,
		Default: Palette{
			Base:        []Color{color(10)},
			Accents:     []Color{color(20)},
			Backgrounds: []Color{color(30)},
		},
	}}

	first, err := validateParams(defs, nil)
	if err != nil {
		t.Fatal(err)
	}
	firstPalette := first["palette"].(Palette)
	firstPalette.Base[0] = color(99)
	firstPalette.Accents[0] = color(98)
	firstPalette.Backgrounds[0] = color(97)

	second, err := validateParams(defs, nil)
	if err != nil {
		t.Fatal(err)
	}
	secondPalette := second["palette"].(Palette)
	if secondPalette.Base[0] == color(99) ||
		secondPalette.Accents[0] == color(98) ||
		secondPalette.Backgrounds[0] == color(97) {
		t.Fatalf("validateParams shared default palette slices: %#v", secondPalette)
	}
}

func TestNewRejectsUnsupportedDeviceKind(t *testing.T) {
	_, err := New(Config{ID: EffectSolid}, Capabilities{LightType: device.LightType(99)})
	if !errors.Is(err, ErrUnsupportedDeviceKind) {
		t.Fatalf("error = %v, want %v", err, ErrUnsupportedDeviceKind)
	}
}

func TestNewRejectsInvalidParams(t *testing.T) {
	tests := map[string]Config{
		"unknown param": {
			ID: EffectSolid,
			Params: map[string]any{
				"unknown": true,
			},
		},
		"invalid color type": {
			ID: EffectSolid,
			Params: map[string]any{
				"color": "red",
			},
		},
		"invalid color value": {
			ID: EffectSolid,
			Params: map[string]any{
				"color": Color{Hue: 999, Saturation: 100, Brightness: 100, Kelvin: 3500},
			},
		},
		"invalid palette value": {
			ID: EffectGradient,
			Params: map[string]any{
				"palette": Palette{Base: []Color{{Hue: 10, Saturation: -1, Brightness: 100, Kelvin: 3500}}},
			},
		},
	}

	for name, config := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := New(config, Capabilities{LightType: device.LightTypeSingleZone}); !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("error = %v, want %v", err, ErrInvalidConfig)
			}
		})
	}
}

func TestValidateParamsRejectsMissingRangeAndChoiceErrors(t *testing.T) {
	minValue := 1.0
	maxValue := 5.0
	defs := []ParamDefinition{
		{Key: "required", Kind: ParamBool, Required: true},
		{Key: "number", Kind: ParamNumber, Min: &minValue, Max: &maxValue},
		{Key: "choice", Kind: ParamChoiceKind, Choices: []ParamChoice{
			{Value: "one", Label: "One"},
			{Value: "two", Label: "Two"},
		}},
	}

	tests := map[string]map[string]any{
		"missing required": {
			"number": 2,
			"choice": "one",
		},
		"below min": {
			"required": true,
			"number":   0,
			"choice":   "one",
		},
		"above max": {
			"required": true,
			"number":   6,
			"choice":   "one",
		},
		"invalid choice": {
			"required": true,
			"number":   2,
			"choice":   "three",
		},
	}

	for name, params := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := validateParams(defs, params); !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("error = %v, want %v", err, ErrInvalidConfig)
			}
		})
	}
}

func TestNewAppliesDefaults(t *testing.T) {
	effect, err := New(Config{ID: EffectGradient}, Capabilities{
		LightType: device.LightTypeMatrix,
		Width:     2,
		Height:    1,
	})
	if err != nil {
		t.Fatal(err)
	}

	frame, ok := effect.Next(time.Second)
	if !ok {
		t.Fatal("Next returned ok=false")
	}
	want := Frame{
		Colors:   []Color{DefaultColor, DefaultColor},
		Width:    2,
		Height:   1,
		Duration: time.Second,
	}
	if !reflect.DeepEqual(frame, want) {
		t.Fatalf("frame = %#v, want %#v", frame, want)
	}
}

func TestConfigJSONRoundTrips(t *testing.T) {
	config := Config{
		ID: EffectSweep,
		Params: map[string]any{
			"palette": Palette{
				Name:        "test",
				Accents:     []Color{color(90)},
				Backgrounds: []Color{color(200)},
			},
		},
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	var got Config
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != config.ID {
		t.Fatalf("ID = %q, want %q", got.ID, config.ID)
	}

	caps := Capabilities{LightType: device.LightTypeMultiZone, Zones: 2}
	direct, err := New(config, caps)
	if err != nil {
		t.Fatal(err)
	}
	roundTripped, err := New(got, caps)
	if err != nil {
		t.Fatal(err)
	}
	directFrame, ok := direct.Next(time.Second)
	if !ok {
		t.Fatal("Next returned ok=false")
	}
	roundTrippedFrame, ok := roundTripped.Next(time.Second)
	if !ok {
		t.Fatal("Next returned ok=false")
	}
	if !reflect.DeepEqual(roundTrippedFrame, directFrame) {
		t.Fatalf("round-tripped frame = %#v, want %#v", roundTrippedFrame, directFrame)
	}
}

func TestParamHelpersReadGoValuesAndJSONValues(t *testing.T) {
	params := map[string]any{
		"color":    color(10),
		"palette":  Palette{Base: []Color{color(20)}, Accents: []Color{color(30)}},
		"number":   12.5,
		"bool":     true,
		"choice":   "forward",
		"duration": 2 * time.Second,
	}

	assertParamHelpers(t, params)

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	var roundTripped map[string]any
	if err := json.Unmarshal(data, &roundTripped); err != nil {
		t.Fatal(err)
	}

	assertParamHelpers(t, roundTripped)
}

func assertParamHelpers(t *testing.T, params map[string]any) {
	t.Helper()

	gotColor, err := ColorParam(params, "color")
	if err != nil {
		t.Fatal(err)
	}
	if gotColor != color(10) {
		t.Fatalf("ColorParam = %#v, want %#v", gotColor, color(10))
	}

	gotPalette, err := PaletteParam(params, "palette")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotPalette.Base, []Color{color(20)}) || !reflect.DeepEqual(gotPalette.Accents, []Color{color(30)}) {
		t.Fatalf("PaletteParam = %#v", gotPalette)
	}

	gotNumber, err := NumberParam(params, "number")
	if err != nil {
		t.Fatal(err)
	}
	if gotNumber != 12.5 {
		t.Fatalf("NumberParam = %f, want 12.5", gotNumber)
	}

	gotBool, err := BoolParam(params, "bool")
	if err != nil {
		t.Fatal(err)
	}
	if !gotBool {
		t.Fatal("BoolParam = false, want true")
	}

	gotChoice, err := ChoiceParam(params, "choice")
	if err != nil {
		t.Fatal(err)
	}
	if gotChoice != "forward" {
		t.Fatalf("ChoiceParam = %q, want forward", gotChoice)
	}

	gotDuration, err := DurationParam(params, "duration")
	if err != nil {
		t.Fatal(err)
	}
	if gotDuration != 2*time.Second {
		t.Fatalf("DurationParam = %s, want 2s", gotDuration)
	}
}

func TestParamHelpersReturnClonedPalette(t *testing.T) {
	original := Palette{Base: []Color{color(10)}, Accents: []Color{color(20)}, Backgrounds: []Color{color(30)}}
	params := map[string]any{"palette": original}

	got, err := PaletteParam(params, "palette")
	if err != nil {
		t.Fatal(err)
	}
	got.Base[0] = color(99)
	got.Accents[0] = color(98)
	got.Backgrounds[0] = color(97)

	if original.Base[0] == color(99) || original.Accents[0] == color(98) || original.Backgrounds[0] == color(97) {
		t.Fatalf("PaletteParam returned shared slices: %#v", original)
	}
}

func TestParamHelpersRejectMissingAndInvalidValues(t *testing.T) {
	tests := map[string]func() error{
		"missing color": func() error {
			_, err := ColorParam(nil, "color")
			return err
		},
		"invalid color": func() error {
			_, err := ColorParam(map[string]any{"color": "blue"}, "color")
			return err
		},
		"invalid palette": func() error {
			_, err := PaletteParam(map[string]any{"palette": "bright"}, "palette")
			return err
		},
		"invalid number": func() error {
			_, err := NumberParam(map[string]any{"number": "12"}, "number")
			return err
		},
		"invalid bool": func() error {
			_, err := BoolParam(map[string]any{"bool": "true"}, "bool")
			return err
		},
		"invalid choice": func() error {
			_, err := ChoiceParam(map[string]any{"choice": 1}, "choice")
			return err
		},
		"invalid duration": func() error {
			_, err := DurationParam(map[string]any{"duration": true}, "duration")
			return err
		},
	}

	for name, run := range tests {
		t.Run(name, func(t *testing.T) {
			if err := run(); !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("error = %v, want %v", err, ErrInvalidConfig)
			}
		})
	}
}

func TestConfigAndDefinitionHaveNoTargetFields(t *testing.T) {
	for _, typ := range []reflect.Type{reflect.TypeOf(Config{}), reflect.TypeOf(EffectDefinition{})} {
		for _, name := range []string{"Target", "Serial", "Group", "Address", "DeviceID", "Scheduler", "Timeline"} {
			if _, ok := typ.FieldByName(name); ok {
				t.Fatalf("%s should not contain target field %q", typ.Name(), name)
			}
		}
	}
	if _, ok := reflect.TypeOf(Config{}).FieldByName("Label"); ok {
		t.Fatal("Config should not contain a label field")
	}
}
