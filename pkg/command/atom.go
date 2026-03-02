package command

import (
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

// intentAtomKind represents the reduced semantic category of a parsed token.
type intentAtomKind int

const (
	intentAtomUnknown intentAtomKind = iota
	intentAtomSelector
	intentAtomAction
	intentAtomSeparator
)

// intentAtom is the smallest semantic unit produced by the parser after
// token normalization.
type intentAtom struct {
	Kind intentAtomKind

	Targets []*device.Device
	Action  *action

	PrevKind, NextKind intentAtomKind
}

func (i *intentAtom) setPower(v bool) {
	if i.Action == nil {
		i.Action = new(action)
	}
	i.Action.Power = &v
}

func (i *intentAtom) setHue(v float64) {
	if i.Action == nil {
		i.Action = new(action)
	}
	i.Action.Hue = &v
}

func (i *intentAtom) setBrightness(v float64) {
	if i.Action == nil {
		i.Action = new(action)
	}
	i.Action.Brightness = &v
}

func (i *intentAtom) setSaturation(v float64) {
	if i.Action == nil {
		i.Action = new(action)
	}
	i.Action.Saturation = &v
}

func (i *intentAtom) setKelvin(v uint16) {
	if i.Action == nil {
		i.Action = new(action)
	}
	i.Action.Kelvin = &v
}

func (i *intentAtom) setDuration(v time.Duration) {
	if i.Action == nil {
		i.Action = new(action)
	}
	i.Action.Duration = &v
}

// buildIntentAtoms converts lexical tokens into semantic atoms.
//
// This phase resolves:
//   - selector lookups into device targets
//   - property/value pairs into concrete actions
//   - duration and color keywords into normalized action fields
//
// The output preserves original ordering but reduces the grammar to only
// three concepts: selector, action, separator.
//
// It also annotates each atom with its previous and next kind so the
// intent builder can later determine how atoms should group together
// (e.g. whether an action starts a new command or extends the previous one).
func (p *CommandParser) buildIntentAtoms(tokens []token) []intentAtom {
	var intentAtoms []intentAtom

	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		var a intentAtom

		switch t.Kind {
		case tokenSelector:
			a.Kind = intentAtomSelector
			a.Targets = p.selectors[t.Raw]

		case tokenColor:
			a.Kind = intentAtomAction
			colorWords[t.Raw](&a)

		case tokenPower:
			a.Kind = intentAtomAction
			a.setPower(t.Raw == "on")

		case tokenProperty:
			if nextToken, ok := peek(tokens, i+1); ok && nextToken.Kind == tokenNumber {
				a.Kind = intentAtomAction
				propertyWords[t.Raw](nextToken.Value, &a)
				i++
			}

		case tokenNumber:
			if nextToken, ok := peek(tokens, i+1); ok {
				switch nextToken.Kind {
				case tokenProperty:
					a.Kind = intentAtomAction
					propertyWords[nextToken.Raw](t.Value, &a)
					i++
				case tokenDuration:
					a.Kind = intentAtomAction
					durationWords[nextToken.Raw](t.Value, &a)
					i++
				}
			}

		case tokenNumberD:
			if f, ok := durationWords[t.Suffix]; ok {
				a.Kind = intentAtomAction
				f(t.Value, &a)
			}

		case tokenNumberK:
			if f, ok := propertyWords[t.Suffix]; ok {
				a.Kind = intentAtomAction
				f(t.Value, &a)
			}

		case tokenSeparator:
			a.Kind = intentAtomSeparator
		}

		if a.Kind != intentAtomUnknown {
			if len(intentAtoms) > 0 {
				a.PrevKind = intentAtoms[len(intentAtoms)-1].Kind
				intentAtoms[len(intentAtoms)-1].NextKind = a.Kind
			}
			intentAtoms = append(intentAtoms, a)
		}
	}

	return intentAtoms
}
