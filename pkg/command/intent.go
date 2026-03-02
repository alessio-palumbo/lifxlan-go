package command

import (
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

// intent represents a fully resolved command.
type intent struct {
	Targets []*device.Device
	Action  *action
}

// action contain a set of optional command actions.
type action struct {
	Power      *bool
	Hue        *float64
	Brightness *float64
	Saturation *float64
	Kelvin     *uint16
	Duration   *time.Duration
}

// Mergeaction applies an action to any unset field
func (m *intent) Mergeaction(a *action) {
	if m.Action == nil {
		m.Action = a
	}
	if m.Action.Power == nil && a.Power != nil {
		m.Action.Power = a.Power
	}
	if m.Action.Hue == nil && a.Hue != nil {
		m.Action.Hue = a.Hue
	}
	if m.Action.Brightness == nil && a.Brightness != nil {
		m.Action.Brightness = a.Brightness
	}
	if m.Action.Saturation == nil && a.Saturation != nil {
		m.Action.Saturation = a.Saturation
	}
	if m.Action.Kelvin == nil && a.Kelvin != nil {
		m.Action.Kelvin = a.Kelvin
	}
	if m.Action.Duration == nil && a.Duration != nil {
		m.Action.Duration = a.Duration
	}
}

// buildIntent groups semantic atoms into executable intents.
//
// This stage determines how selectors and actions bind together based on
// ordering and neighbour relationships. For example:
//
// Incomplete trailing data is merged into the previous intent when possible,
// allowing forgiving parsing of natural language commands.
func (p *CommandParser) buildIntent(atoms []intentAtom) []intent {
	pending := new(intent)
	var intents []intent

	flush := func(ifTrue bool) {
		if ifTrue && pending.Action != nil && len(pending.Targets) > 0 {
			intents = append(intents, *pending)
			pending = new(intent)
		}
	}

	for i := range atoms {
		a := atoms[i]

		switch a.Kind {
		case intentAtomSelector:
			flush(a.PrevKind != intentAtomSelector)
			pending.Targets = append(pending.Targets, a.Targets...)

		case intentAtomAction:
			flush(a.NextKind == intentAtomSelector)
			pending.Mergeaction(a.Action)
		case intentAtomSeparator:
			flush(true)
		}
	}

	// Append last complete intent otherwise merge Action or Targets with previous one.
	if pending.Action != nil && len(pending.Targets) > 0 {
		intents = append(intents, *pending)
	} else if len(intents) > 0 {
		last := &intents[len(intents)-1]
		if pending.Action != nil {
			last.Mergeaction(pending.Action)
		} else if len(pending.Targets) > 0 {
			last.Targets = append(last.Targets, pending.Targets...)
		}
	}

	return intents
}

// peek looks up the next token, if available.
func peek(tokens []token, i int) (token, bool) {
	if i >= len(tokens) {
		return token{}, false
	}
	return tokens[i], true
}
