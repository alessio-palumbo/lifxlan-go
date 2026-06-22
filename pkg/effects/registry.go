package effects

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

// EffectID identifies a registered effect.
type EffectID string

// ParamKind identifies a configurable parameter type.
type ParamKind string

const (
	// ParamNumber is a numeric parameter.
	ParamNumber ParamKind = "number"
	// ParamBool is a boolean parameter.
	ParamBool ParamKind = "bool"
	// ParamChoiceKind is a string choice parameter.
	ParamChoiceKind ParamKind = "choice"
	// ParamColor is a Color parameter.
	ParamColor ParamKind = "color"
	// ParamPalette is a Palette parameter.
	ParamPalette ParamKind = "palette"
	// ParamDuration is a time.Duration parameter.
	ParamDuration ParamKind = "duration"
)

const (
	// EffectSolid identifies the Solid effect.
	EffectSolid EffectID = "solid"
	// EffectGradient identifies the Gradient effect.
	EffectGradient EffectID = "gradient"
	// EffectSweep identifies the Sweep effect.
	EffectSweep EffectID = "sweep"
	// EffectWaterfall identifies the Waterfall matrix effect.
	EffectWaterfall EffectID = "waterfall"
	// EffectRockets identifies the Rockets matrix effect.
	EffectRockets EffectID = "rockets"
	// EffectSnake identifies the Snake matrix effect.
	EffectSnake EffectID = "snake"
	// EffectWorm identifies the Worm matrix effect.
	EffectWorm EffectID = "worm"
	// EffectWave identifies the Wave matrix effect.
	EffectWave EffectID = "wave"
	// EffectConcentricFrames identifies the ConcentricFrames matrix effect.
	EffectConcentricFrames EffectID = "concentric_frames"
)

var (
	// ErrUnknownEffect is returned when an effect ID is not registered.
	ErrUnknownEffect = errors.New("unknown effect")
	// ErrDuplicateEffect is returned when registering the same effect ID twice.
	ErrDuplicateEffect = errors.New("duplicate effect")
	// ErrInvalidDefinition is returned when an effect definition is invalid.
	ErrInvalidDefinition = errors.New("invalid effect definition")
	// ErrUnsupportedDeviceKind is returned when an effect does not support capabilities.
	ErrUnsupportedDeviceKind = errors.New("unsupported device kind")
	// ErrInvalidConfig is returned when an effect config is invalid.
	ErrInvalidConfig = errors.New("invalid effect config")
)

// EffectDefinition describes a registered effect and its configurable params.
type EffectDefinition struct {
	ID          EffectID
	Label       string
	Description string
	DeviceKinds []device.LightType
	Params      []ParamDefinition
	New         func(Config, Capabilities) (Effect, error)
}

// ParamDefinition describes one effect configuration parameter.
type ParamDefinition struct {
	Key      string
	Label    string
	Kind     ParamKind
	Default  any
	Min      *float64
	Max      *float64
	Step     *float64
	Choices  []ParamChoice
	Required bool
}

// ParamChoice describes one allowed choice value.
type ParamChoice struct {
	Value string
	Label string
}

// Config is a serializable, target-free effect configuration.
type Config struct {
	ID     EffectID       `json:"id"`
	Params map[string]any `json:"params,omitempty"`
}

var (
	registryMu sync.RWMutex
	registry   = map[EffectID]EffectDefinition{}
)

func init() {
	mustRegister(EffectDefinition{
		ID:          EffectSolid,
		Label:       "Solid",
		Description: "Fill every logical zone or pixel with one color.",
		DeviceKinds: allLightTypes(),
		Params: []ParamDefinition{
			{
				Key:     "color",
				Label:   "Color",
				Kind:    ParamColor,
				Default: DefaultColor,
			},
		},
		New: func(config Config, caps Capabilities) (Effect, error) {
			color, err := colorParam(config.Params, "color")
			if err != nil {
				return nil, err
			}
			return NewSolid(SolidConfig{Capabilities: caps, Color: color}), nil
		},
	})

	defaultPalette := Palette{Base: []Color{DefaultColor}}
	mustRegister(EffectDefinition{
		ID:          EffectGradient,
		Label:       "Gradient",
		Description: "Fill the logical surface with deterministic palette stops.",
		DeviceKinds: allLightTypes(),
		Params: []ParamDefinition{
			{
				Key:     "palette",
				Label:   "Palette",
				Kind:    ParamPalette,
				Default: defaultPalette,
			},
		},
		New: func(config Config, caps Capabilities) (Effect, error) {
			palette, err := paletteParam(config.Params, "palette")
			if err != nil {
				return nil, err
			}
			return NewGradient(GradientConfig{Capabilities: caps, Palette: palette}), nil
		},
	})

	mustRegister(EffectDefinition{
		ID:          EffectSweep,
		Label:       "Sweep",
		Description: "Move accent colors across a background frame.",
		DeviceKinds: allLightTypes(),
		Params: []ParamDefinition{
			{
				Key:     "palette",
				Label:   "Palette",
				Kind:    ParamPalette,
				Default: defaultPalette,
			},
		},
		New: func(config Config, caps Capabilities) (Effect, error) {
			palette, err := paletteParam(config.Params, "palette")
			if err != nil {
				return nil, err
			}
			return NewSweep(SweepConfig{Capabilities: caps, Palette: palette}), nil
		},
	})

	mustRegister(EffectDefinition{
		ID:          EffectWaterfall,
		Label:       "Waterfall",
		Description: "Fill matrix rows cumulatively with a centered color strip.",
		DeviceKinds: matrixLightTypes(),
		Params: []ParamDefinition{
			paletteParamDefinition(defaultPalette),
			cyclesParamDefinition(),
		},
		New: func(config Config, caps Capabilities) (Effect, error) {
			palette, err := paletteParam(config.Params, "palette")
			if err != nil {
				return nil, err
			}
			cycles, err := intParam(config.Params, "cycles")
			if err != nil {
				return nil, err
			}
			return NewWaterfall(WaterfallConfig{Capabilities: caps, Colors: paletteColors(palette), Cycles: cycles}), nil
		},
	})

	mustRegister(EffectDefinition{
		ID:          EffectRockets,
		Label:       "Rockets",
		Description: "Move a single pixel through the matrix in row-major order.",
		DeviceKinds: matrixLightTypes(),
		Params: []ParamDefinition{
			paletteParamDefinition(defaultPalette),
			cyclesParamDefinition(),
		},
		New: func(config Config, caps Capabilities) (Effect, error) {
			palette, err := paletteParam(config.Params, "palette")
			if err != nil {
				return nil, err
			}
			cycles, err := intParam(config.Params, "cycles")
			if err != nil {
				return nil, err
			}
			return NewRockets(RocketsConfig{Capabilities: caps, Colors: paletteColors(palette), Cycles: cycles}), nil
		},
	})

	mustRegister(EffectDefinition{
		ID:          EffectSnake,
		Label:       "Snake",
		Description: "Move a trailing segment through a serpentine matrix path.",
		DeviceKinds: matrixLightTypes(),
		Params: []ParamDefinition{
			colorParamDefinition(DefaultColor),
			sizeParamDefinition(),
			cyclesParamDefinition(),
		},
		New: func(config Config, caps Capabilities) (Effect, error) {
			color, err := colorParam(config.Params, "color")
			if err != nil {
				return nil, err
			}
			size, err := intParam(config.Params, "size")
			if err != nil {
				return nil, err
			}
			cycles, err := intParam(config.Params, "cycles")
			if err != nil {
				return nil, err
			}
			return NewSnake(SnakeConfig{Capabilities: caps, Size: size, Color: color, Cycles: cycles}), nil
		},
	})

	mustRegister(EffectDefinition{
		ID:          EffectWorm,
		Label:       "Worm",
		Description: "Move short batches of pixels through a serpentine matrix path.",
		DeviceKinds: matrixLightTypes(),
		Params: []ParamDefinition{
			colorParamDefinition(DefaultColor),
			sizeParamDefinition(),
			cyclesParamDefinition(),
		},
		New: func(config Config, caps Capabilities) (Effect, error) {
			color, err := colorParam(config.Params, "color")
			if err != nil {
				return nil, err
			}
			size, err := intParam(config.Params, "size")
			if err != nil {
				return nil, err
			}
			cycles, err := intParam(config.Params, "cycles")
			if err != nil {
				return nil, err
			}
			return NewWorm(WormConfig{Capabilities: caps, Size: size, Color: color, Cycles: cycles}), nil
		},
	})

	mustRegister(EffectDefinition{
		ID:          EffectWave,
		Label:       "Wave",
		Description: "Displace matrix columns upward as a wave front crosses the frame.",
		DeviceKinds: matrixLightTypes(),
		Params: []ParamDefinition{
			paletteParamDefinition(Palette{}),
			amplitudeParamDefinition(),
			waveWidthParamDefinition(),
			wavesParamDefinition(),
			cyclesParamDefinition(),
		},
		New: func(config Config, caps Capabilities) (Effect, error) {
			palette, err := paletteParam(config.Params, "palette")
			if err != nil {
				return nil, err
			}
			amplitude, err := intParam(config.Params, "amplitude")
			if err != nil {
				return nil, err
			}
			width, err := intParam(config.Params, "width")
			if err != nil {
				return nil, err
			}
			waves, err := intParam(config.Params, "waves")
			if err != nil {
				return nil, err
			}
			cycles, err := intParam(config.Params, "cycles")
			if err != nil {
				return nil, err
			}
			return NewWave(WaveConfig{Capabilities: caps, Palette: palette, Amplitude: amplitude, Width: width, Waves: waves, Cycles: cycles}), nil
		},
	})

	mustRegister(EffectDefinition{
		ID:          EffectConcentricFrames,
		Label:       "Concentric Frames",
		Description: "Draw matrix borders according to direction.",
		DeviceKinds: matrixLightTypes(),
		Params: []ParamDefinition{
			paletteParamDefinition(defaultPalette),
			directionParamDefinition(),
			cyclesParamDefinition(),
		},
		New: func(config Config, caps Capabilities) (Effect, error) {
			palette, err := paletteParam(config.Params, "palette")
			if err != nil {
				return nil, err
			}
			direction, err := directionParam(config.Params, "direction")
			if err != nil {
				return nil, err
			}
			cycles, err := intParam(config.Params, "cycles")
			if err != nil {
				return nil, err
			}
			return NewConcentricFrames(ConcentricFramesConfig{Capabilities: caps, Direction: direction, Colors: paletteColors(palette), Cycles: cycles}), nil
		},
	})
}

// Register adds def to the global effect registry.
func Register(def EffectDefinition) error {
	if err := validateDefinition(def); err != nil {
		return err
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, exists := registry[def.ID]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateEffect, def.ID)
	}
	registry[def.ID] = cloneDefinition(def)
	return nil
}

// Definition returns the definition for id.
func Definition(id EffectID) (EffectDefinition, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	def, ok := registry[id]
	if !ok {
		return EffectDefinition{}, false
	}
	return cloneDefinition(def), true
}

// Definitions returns registered definitions in deterministic ID order.
func Definitions() []EffectDefinition {
	registryMu.RLock()
	defer registryMu.RUnlock()
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)

	defs := make([]EffectDefinition, 0, len(ids))
	for _, id := range ids {
		defs = append(defs, cloneDefinition(registry[EffectID(id)]))
	}
	return defs
}

// New validates config and returns a configured effect for caps.
func New(config Config, caps Capabilities) (Effect, error) {
	registryMu.RLock()
	def, ok := registry[config.ID]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownEffect, config.ID)
	}
	if !supportsLightType(def.DeviceKinds, caps.LightType) {
		return nil, fmt.Errorf("%w: %s does not support %s", ErrUnsupportedDeviceKind, config.ID, caps.LightType)
	}

	params, err := validateParams(def.Params, config.Params)
	if err != nil {
		return nil, err
	}
	return def.New(Config{ID: config.ID, Params: params}, caps)
}

func mustRegister(def EffectDefinition) {
	if err := Register(def); err != nil {
		panic(err)
	}
}

func validateDefinition(def EffectDefinition) error {
	if def.ID == "" {
		return fmt.Errorf("%w: missing ID", ErrInvalidDefinition)
	}
	if def.Label == "" {
		return fmt.Errorf("%w: missing label for %s", ErrInvalidDefinition, def.ID)
	}
	if def.New == nil {
		return fmt.Errorf("%w: missing constructor for %s", ErrInvalidDefinition, def.ID)
	}

	seen := map[string]bool{}
	for _, param := range def.Params {
		if param.Key == "" {
			return fmt.Errorf("%w: empty parameter key for %s", ErrInvalidDefinition, def.ID)
		}
		if seen[param.Key] {
			return fmt.Errorf("%w: duplicate parameter %q for %s", ErrInvalidDefinition, param.Key, def.ID)
		}
		seen[param.Key] = true
		if !knownParamKind(param.Kind) {
			return fmt.Errorf("%w: unknown parameter kind %q for %s", ErrInvalidDefinition, param.Kind, def.ID)
		}
	}
	return nil
}

func validateParams(defs []ParamDefinition, params map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(defs))
	known := make(map[string]ParamDefinition, len(defs))
	for _, def := range defs {
		known[def.Key] = def
		if def.Default != nil {
			out[def.Key] = cloneParamDefault(def.Default)
		}
	}

	for key := range params {
		if _, ok := known[key]; !ok {
			return nil, fmt.Errorf("%w: unknown parameter %q", ErrInvalidConfig, key)
		}
	}

	for _, def := range defs {
		value, ok := params[def.Key]
		if !ok {
			if def.Required && def.Default == nil {
				return nil, fmt.Errorf("%w: missing required parameter %q", ErrInvalidConfig, def.Key)
			}
			value = cloneParamDefault(def.Default)
		}
		if value == nil {
			if def.Required {
				return nil, fmt.Errorf("%w: missing required parameter %q", ErrInvalidConfig, def.Key)
			}
			continue
		}
		normalized, err := validateParamValue(def, value)
		if err != nil {
			return nil, err
		}
		out[def.Key] = normalized
	}
	return out, nil
}

func validateParamValue(def ParamDefinition, value any) (any, error) {
	switch def.Kind {
	case ParamNumber:
		number, err := numberValue(value)
		if err != nil {
			return nil, fmt.Errorf("%w: parameter %q must be a number", ErrInvalidConfig, def.Key)
		}
		if def.Min != nil && number < *def.Min {
			return nil, fmt.Errorf("%w: parameter %q below minimum", ErrInvalidConfig, def.Key)
		}
		if def.Max != nil && number > *def.Max {
			return nil, fmt.Errorf("%w: parameter %q above maximum", ErrInvalidConfig, def.Key)
		}
		return number, nil
	case ParamBool:
		boolValue, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("%w: parameter %q must be a bool", ErrInvalidConfig, def.Key)
		}
		return boolValue, nil
	case ParamChoiceKind:
		choice, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%w: parameter %q must be a choice string", ErrInvalidConfig, def.Key)
		}
		for _, allowed := range def.Choices {
			if choice == allowed.Value {
				return choice, nil
			}
		}
		return nil, fmt.Errorf("%w: parameter %q has invalid choice %q", ErrInvalidConfig, def.Key, choice)
	case ParamColor:
		color, err := asColor(value)
		if err != nil {
			return nil, fmt.Errorf("%w: parameter %q: %v", ErrInvalidConfig, def.Key, err)
		}
		return color, nil
	case ParamPalette:
		palette, err := asPalette(value)
		if err != nil {
			return nil, fmt.Errorf("%w: parameter %q: %v", ErrInvalidConfig, def.Key, err)
		}
		return palette, nil
	case ParamDuration:
		duration, err := asDuration(value)
		if err != nil {
			return nil, fmt.Errorf("%w: parameter %q must be a duration", ErrInvalidConfig, def.Key)
		}
		return duration, nil
	default:
		return nil, fmt.Errorf("%w: unknown parameter kind %q", ErrInvalidConfig, def.Kind)
	}
}

// ColorParam returns a validated color parameter.
func ColorParam(params map[string]any, key string) (Color, error) {
	value, err := requiredParam(params, key)
	if err != nil {
		return Color{}, err
	}
	color, err := asColor(value)
	if err != nil {
		return Color{}, fmt.Errorf("%w: parameter %q: %v", ErrInvalidConfig, key, err)
	}
	return color, nil
}

// PaletteParam returns a validated palette parameter.
func PaletteParam(params map[string]any, key string) (Palette, error) {
	value, err := requiredParam(params, key)
	if err != nil {
		return Palette{}, err
	}
	palette, err := asPalette(value)
	if err != nil {
		return Palette{}, fmt.Errorf("%w: parameter %q: %v", ErrInvalidConfig, key, err)
	}
	return palette, nil
}

// NumberParam returns a validated number parameter.
func NumberParam(params map[string]any, key string) (float64, error) {
	value, err := requiredParam(params, key)
	if err != nil {
		return 0, err
	}
	number, err := numberValue(value)
	if err != nil {
		return 0, fmt.Errorf("%w: parameter %q must be a number", ErrInvalidConfig, key)
	}
	return number, nil
}

// BoolParam returns a validated boolean parameter.
func BoolParam(params map[string]any, key string) (bool, error) {
	value, err := requiredParam(params, key)
	if err != nil {
		return false, err
	}
	boolValue, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("%w: parameter %q must be a bool", ErrInvalidConfig, key)
	}
	return boolValue, nil
}

// ChoiceParam returns a validated string choice parameter.
func ChoiceParam(params map[string]any, key string) (string, error) {
	value, err := requiredParam(params, key)
	if err != nil {
		return "", err
	}
	choice, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%w: parameter %q must be a choice string", ErrInvalidConfig, key)
	}
	return choice, nil
}

// DurationParam returns a validated duration parameter.
func DurationParam(params map[string]any, key string) (time.Duration, error) {
	value, err := requiredParam(params, key)
	if err != nil {
		return 0, err
	}
	duration, err := asDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%w: parameter %q must be a duration", ErrInvalidConfig, key)
	}
	return duration, nil
}

func colorParam(params map[string]any, key string) (Color, error) {
	return ColorParam(params, key)
}

func paletteParam(params map[string]any, key string) (Palette, error) {
	return PaletteParam(params, key)
}

func intParam(params map[string]any, key string) (int, error) {
	number, err := NumberParam(params, key)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(number) || math.IsInf(number, 0) || math.Trunc(number) != number {
		return 0, fmt.Errorf("%w: parameter %q must be an integer", ErrInvalidConfig, key)
	}
	return int(number), nil
}

func directionParam(params map[string]any, key string) (Direction, error) {
	choice, err := ChoiceParam(params, key)
	if err != nil {
		return DirectionInwards, err
	}
	switch choice {
	case "inwards":
		return DirectionInwards, nil
	case "outwards":
		return DirectionOutwards, nil
	case "in_out":
		return DirectionInOut, nil
	case "out_in":
		return DirectionOutIn, nil
	default:
		return DirectionInwards, fmt.Errorf("%w: parameter %q has invalid choice %q", ErrInvalidConfig, key, choice)
	}
}

func requiredParam(params map[string]any, key string) (any, error) {
	value, ok := params[key]
	if !ok || value == nil {
		return nil, fmt.Errorf("%w: missing parameter %q", ErrInvalidConfig, key)
	}
	return value, nil
}

func asColor(value any) (Color, error) {
	switch v := value.(type) {
	case Color:
		return validateColor(v)
	case map[string]any:
		color := Color{}
		if err := setFloatField(v, "Hue", "hue", &color.Hue); err != nil {
			return Color{}, err
		}
		if err := setFloatField(v, "Saturation", "saturation", &color.Saturation); err != nil {
			return Color{}, err
		}
		if err := setFloatField(v, "Brightness", "brightness", &color.Brightness); err != nil {
			return Color{}, err
		}
		kelvin, err := optionalFloatField(v, "Kelvin", "kelvin")
		if err != nil {
			return Color{}, err
		}
		if kelvin < 0 || kelvin > math.MaxUint16 {
			return Color{}, fmt.Errorf("kelvin out of range")
		}
		color.Kelvin = uint16(kelvin)
		return validateColor(color)
	default:
		return Color{}, fmt.Errorf("must be a color")
	}
}

func validateColor(color Color) (Color, error) {
	if !finiteInRange(color.Hue, 0, 360) {
		return Color{}, fmt.Errorf("hue must be in [0, 360]")
	}
	if !finiteInRange(color.Saturation, 0, 100) {
		return Color{}, fmt.Errorf("saturation must be in [0, 100]")
	}
	if !finiteInRange(color.Brightness, 0, 100) {
		return Color{}, fmt.Errorf("brightness must be in [0, 100]")
	}
	if color.Kelvin != 0 && (color.Kelvin < 1500 || color.Kelvin > 9000) {
		return Color{}, fmt.Errorf("kelvin must be 0 or in [1500, 9000]")
	}
	return color, nil
}

func asPalette(value any) (Palette, error) {
	switch v := value.(type) {
	case Palette:
		return validatePalette(clonePalette(v))
	case map[string]any:
		palette := Palette{}
		if name, ok := stringField(v, "Name", "name"); ok {
			palette.Name = name
		}
		var err error
		if palette.Base, err = colorSliceField(v, "Base", "base"); err != nil {
			return Palette{}, err
		}
		if palette.Accents, err = colorSliceField(v, "Accents", "accents"); err != nil {
			return Palette{}, err
		}
		if palette.Backgrounds, err = colorSliceField(v, "Backgrounds", "backgrounds"); err != nil {
			return Palette{}, err
		}
		return validatePalette(palette)
	default:
		return Palette{}, fmt.Errorf("must be a palette")
	}
}

func validatePalette(palette Palette) (Palette, error) {
	for _, colors := range [][]Color{palette.Base, palette.Accents, palette.Backgrounds} {
		for _, color := range colors {
			if _, err := validateColor(color); err != nil {
				return Palette{}, err
			}
		}
	}
	return palette, nil
}

func asDuration(value any) (time.Duration, error) {
	switch v := value.(type) {
	case time.Duration:
		return v, nil
	case string:
		return time.ParseDuration(v)
	case float64:
		return time.Duration(v), nil
	case int:
		return time.Duration(v), nil
	default:
		return 0, fmt.Errorf("invalid duration")
	}
}

func asFloat64(value any) (float64, error) {
	return numberValue(value)
}

func numberValue(value any) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case json.Number:
		return strconv.ParseFloat(string(v), 64)
	default:
		return 0, fmt.Errorf("invalid number")
	}
}

func setFloatField(values map[string]any, exported, lower string, target *float64) error {
	value, ok := fieldValue(values, exported, lower)
	if !ok {
		return nil
	}
	number, err := asFloat64(value)
	if err != nil {
		return fmt.Errorf("%s must be a number", lower)
	}
	*target = number
	return nil
}

func optionalFloatField(values map[string]any, exported, lower string) (float64, error) {
	value, ok := fieldValue(values, exported, lower)
	if !ok {
		return 0, nil
	}
	number, err := asFloat64(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number", lower)
	}
	return number, nil
}

func stringField(values map[string]any, exported, lower string) (string, bool) {
	value, ok := fieldValue(values, exported, lower)
	if !ok {
		return "", false
	}
	stringValue, _ := value.(string)
	return stringValue, stringValue != ""
}

func colorSliceField(values map[string]any, exported, lower string) ([]Color, error) {
	value, ok := fieldValue(values, exported, lower)
	if !ok {
		return nil, nil
	}
	if value == nil {
		return nil, nil
	}
	switch v := value.(type) {
	case []Color:
		return slices.Clone(v), nil
	case []any:
		colors := make([]Color, len(v))
		for i, item := range v {
			color, err := asColor(item)
			if err != nil {
				return nil, err
			}
			colors[i] = color
		}
		return colors, nil
	default:
		return nil, fmt.Errorf("%s must be a color array", lower)
	}
}

func fieldValue(values map[string]any, exported, lower string) (any, bool) {
	if value, ok := values[exported]; ok {
		return value, true
	}
	value, ok := values[lower]
	return value, ok
}

func finiteInRange(value, minValue, maxValue float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= minValue && value <= maxValue
}

func supportsLightType(kinds []device.LightType, kind device.LightType) bool {
	if len(kinds) == 0 {
		return true
	}
	return slices.Contains(kinds, kind)
}

func allLightTypes() []device.LightType {
	return []device.LightType{device.LightTypeSingleZone, device.LightTypeMultiZone, device.LightTypeMatrix}
}

func matrixLightTypes() []device.LightType {
	return []device.LightType{device.LightTypeMatrix}
}

func knownParamKind(kind ParamKind) bool {
	switch kind {
	case ParamNumber, ParamBool, ParamChoiceKind, ParamColor, ParamPalette, ParamDuration:
		return true
	default:
		return false
	}
}

func colorParamDefinition(defaultColor Color) ParamDefinition {
	return ParamDefinition{
		Key:     "color",
		Label:   "Color",
		Kind:    ParamColor,
		Default: defaultColor,
	}
}

func amplitudeParamDefinition() ParamDefinition {
	return ParamDefinition{
		Key:     "amplitude",
		Label:   "Amplitude",
		Kind:    ParamNumber,
		Default: 2,
		Min:     float64Ptr(1),
		Step:    float64Ptr(1),
	}
}

func waveWidthParamDefinition() ParamDefinition {
	return ParamDefinition{
		Key:     "width",
		Label:   "Width",
		Kind:    ParamNumber,
		Default: 3,
		Min:     float64Ptr(1),
		Step:    float64Ptr(1),
	}
}

func wavesParamDefinition() ParamDefinition {
	return ParamDefinition{
		Key:     "waves",
		Label:   "Waves",
		Kind:    ParamNumber,
		Default: 1,
		Min:     float64Ptr(1),
		Step:    float64Ptr(1),
	}
}

func paletteParamDefinition(defaultPalette Palette) ParamDefinition {
	return ParamDefinition{
		Key:     "palette",
		Label:   "Palette",
		Kind:    ParamPalette,
		Default: defaultPalette,
	}
}

func cyclesParamDefinition() ParamDefinition {
	return ParamDefinition{
		Key:     "cycles",
		Label:   "Cycles",
		Kind:    ParamNumber,
		Default: 0,
		Min:     float64Ptr(0),
		Step:    float64Ptr(1),
	}
}

func sizeParamDefinition() ParamDefinition {
	return ParamDefinition{
		Key:     "size",
		Label:   "Size",
		Kind:    ParamNumber,
		Default: 4,
		Min:     float64Ptr(1),
		Step:    float64Ptr(1),
	}
}

func directionParamDefinition() ParamDefinition {
	return ParamDefinition{
		Key:     "direction",
		Label:   "Direction",
		Kind:    ParamChoiceKind,
		Default: "inwards",
		Choices: []ParamChoice{
			{Value: "inwards", Label: "Inwards"},
			{Value: "outwards", Label: "Outwards"},
			{Value: "in_out", Label: "In Out"},
			{Value: "out_in", Label: "Out In"},
		},
	}
}

func paletteColors(palette Palette) []Color {
	colors := make([]Color, 0, len(palette.Base)+len(palette.Accents))
	colors = append(colors, palette.Base...)
	colors = append(colors, palette.Accents...)
	if len(colors) == 0 {
		return []Color{DefaultColor}
	}
	return colors
}

func float64Ptr(value float64) *float64 {
	return &value
}

func cloneDefinition(def EffectDefinition) EffectDefinition {
	def.DeviceKinds = slices.Clone(def.DeviceKinds)
	def.Params = slices.Clone(def.Params)
	for i := range def.Params {
		def.Params[i].Choices = slices.Clone(def.Params[i].Choices)
		def.Params[i].Default = cloneParamDefault(def.Params[i].Default)
	}
	return def
}

func cloneParamDefault(value any) any {
	switch v := value.(type) {
	case Palette:
		return clonePalette(v)
	case []Color:
		return slices.Clone(v)
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = cloneParamDefault(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = cloneParamDefault(item)
		}
		return out
	default:
		return value
	}
}

func clonePalette(palette Palette) Palette {
	palette.Base = slices.Clone(palette.Base)
	palette.Accents = slices.Clone(palette.Accents)
	palette.Backgrounds = slices.Clone(palette.Backgrounds)
	return palette
}
