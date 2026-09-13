package device

import (
	"slices"
	"time"

	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/enums"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

const maxEffectDuration = time.Duration(1<<63 - 1)

// EffectDirection describes the direction of a multizone firmware effect.
type EffectDirection uint8

const (
	EffectDirectionReverse EffectDirection = iota
	EffectDirectionForward
)

func (d EffectDirection) String() string {
	switch d {
	case EffectDirectionReverse:
		return "reverse"
	case EffectDirectionForward:
		return "forward"
	default:
		return "unknown"
	}
}

// MultizoneEffectType identifies a multizone firmware effect.
type MultizoneEffectType uint8

const (
	MultizoneEffectTypeOff  MultizoneEffectType = MultizoneEffectType(enums.MultiZoneEffectTypeMULTIZONEEFFECTTYPEOFF)
	MultizoneEffectTypeMove MultizoneEffectType = MultizoneEffectType(enums.MultiZoneEffectTypeMULTIZONEEFFECTTYPEMOVE)
)

func (t MultizoneEffectType) String() string {
	switch t {
	case MultizoneEffectTypeOff:
		return "off"
	case MultizoneEffectTypeMove:
		return "move"
	default:
		return "unknown"
	}
}

// MatrixEffectType identifies a matrix firmware effect.
type MatrixEffectType uint8

const (
	MatrixEffectTypeOff        MatrixEffectType = MatrixEffectType(enums.TileEffectTypeTILEEFFECTTYPEOFF)
	MatrixEffectTypeMorph      MatrixEffectType = MatrixEffectType(enums.TileEffectTypeTILEEFFECTTYPEMORPH)
	MatrixEffectTypeFlame      MatrixEffectType = MatrixEffectType(enums.TileEffectTypeTILEEFFECTTYPEFLAME)
	MatrixEffectTypeSky        MatrixEffectType = MatrixEffectType(enums.TileEffectTypeTILEEFFECTTYPESKY)
	MatrixEffectTypeColorSweep MatrixEffectType = MatrixEffectType(enums.TileEffectTypeTILEEFFECTTYPECOLORSWEEP)
)

func (t MatrixEffectType) String() string {
	switch t {
	case MatrixEffectTypeOff:
		return "off"
	case MatrixEffectTypeMorph:
		return "morph"
	case MatrixEffectTypeFlame:
		return "flame"
	case MatrixEffectTypeSky:
		return "sky"
	case MatrixEffectTypeColorSweep:
		return "color_sweep"
	default:
		return "unknown"
	}
}

// MatrixEffectSkyType identifies the variant of a matrix sky effect.
type MatrixEffectSkyType uint8

const (
	MatrixEffectSkyTypeSunrise MatrixEffectSkyType = MatrixEffectSkyType(enums.TileEffectSkyTypeTILEEFFECTSKYTYPESUNRISE)
	MatrixEffectSkyTypeSunset  MatrixEffectSkyType = MatrixEffectSkyType(enums.TileEffectSkyTypeTILEEFFECTSKYTYPESUNSET)
	MatrixEffectSkyTypeClouds  MatrixEffectSkyType = MatrixEffectSkyType(enums.TileEffectSkyTypeTILEEFFECTSKYTYPECLOUDS)
)

func (t MatrixEffectSkyType) String() string {
	switch t {
	case MatrixEffectSkyTypeSunrise:
		return "sunrise"
	case MatrixEffectSkyTypeSunset:
		return "sunset"
	case MatrixEffectSkyTypeClouds:
		return "clouds"
	default:
		return "unknown"
	}
}

// MultizoneEffect is the last observed multizone firmware effect state.
// Known distinguishes an observed OFF state from state that has not been read.
type MultizoneEffect struct {
	Known             bool
	Type              MultizoneEffectType
	InstanceID        uint32
	Speed             time.Duration
	RemainingDuration time.Duration
	Direction         EffectDirection
}

// Running reports whether an observed multizone firmware effect is active.
func (e MultizoneEffect) Running() bool {
	return e.Known && e.Type != MultizoneEffectTypeOff
}

// MatrixEffect is the last observed matrix firmware effect state.
// SkyType is meaningful when Type is MatrixEffectTypeSky. Known distinguishes
// an observed OFF state from state that has not been read.
type MatrixEffect struct {
	Known             bool
	Type              MatrixEffectType
	SkyType           MatrixEffectSkyType
	InstanceID        uint32
	Speed             time.Duration
	RemainingDuration time.Duration
	Palette           []Color
}

// Running reports whether an observed matrix firmware effect is active.
func (e MatrixEffect) Running() bool {
	return e.Known && e.Type != MatrixEffectTypeOff
}

// SetMultizoneEffect caches an observed multizone firmware effect state.
func (d *Device) SetMultizoneEffect(p *packets.MultiZoneStateEffect) bool {
	settings := p.Settings
	effect := MultizoneEffect{
		Known:             true,
		Type:              MultizoneEffectType(settings.Type),
		InstanceID:        settings.Instanceid,
		Speed:             time.Duration(settings.Speed) * time.Millisecond,
		RemainingDuration: effectDuration(settings.Duration),
		Direction:         EffectDirection(settings.Parameter.Parameter1),
	}
	if d.MultizoneProperties.Effect == effect {
		return false
	}
	d.MultizoneProperties.Effect = effect
	return true
}

// SetMatrixEffect caches an observed matrix firmware effect state.
func (d *Device) SetMatrixEffect(p *packets.TileStateEffect) bool {
	settings := p.Settings
	paletteCount := min(int(settings.PaletteCount), len(settings.Palette))
	palette := make([]Color, paletteCount)
	for i := range paletteCount {
		palette[i] = NewColor(settings.Palette[i])
	}

	effect := MatrixEffect{
		Known:             true,
		Type:              MatrixEffectType(settings.Type),
		SkyType:           MatrixEffectSkyType(settings.Parameter.Parameter0),
		InstanceID:        settings.Instanceid,
		Speed:             time.Duration(settings.Speed) * time.Millisecond,
		RemainingDuration: effectDuration(settings.Duration),
		Palette:           palette,
	}
	current := d.MatrixProperties.Effect
	if current.Known == effect.Known && current.Type == effect.Type && current.SkyType == effect.SkyType &&
		current.InstanceID == effect.InstanceID && current.Speed == effect.Speed &&
		current.RemainingDuration == effect.RemainingDuration && slices.Equal(current.Palette, effect.Palette) {
		return false
	}
	d.MatrixProperties.Effect = effect
	return true
}

func effectDuration(nanoseconds uint64) time.Duration {
	if nanoseconds > uint64(maxEffectDuration) {
		return maxEffectDuration
	}
	return time.Duration(nanoseconds)
}
