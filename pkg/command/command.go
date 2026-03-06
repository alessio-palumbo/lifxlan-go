// Package command implements a lightweight natural-language command parser
// for LIFX device control.
//
// The parser converts free-form user input into executable device commands
// through a multi-stage pipeline:
//
//	text → tokens → intentAtoms → intents → protocol messages
//
// Stages:
//
//  1. Lexing (tokens)
//     Raw words are classified into token types such as selectors,
//     colors, numbers, properties, durations, etc.
//
//  2. Semantic reduction (intentAtoms)
//     Tokens are resolved into intentAtoms — a minimal grammar consisting of
//     selectors, actions, and separators. At this stage property/value pairs,
//     colors, and durations are normalized into concrete action fields.
//
//  3. Intent construction (intents)
//     intentAtoms are grouped based on ordering and neighbour relationships to form
//     executable intents. Each intent binds a set of target devices to a
//     single aggregated action.
//
//  4. Command compilation
//     intents are converted into Command values that bundle:
//     - protocol messages to send
//     - target device serials
//     Dispatching is intentionally left to the caller so the parser remains
//     transport-agnostic and reusable.
//
// This layered approach keeps the parser flexible, forgiving of natural
// phrasing, and easy to extend with new keywords or behaviours without
// modifying the core grouping logic.
package command

import (
	"strings"
	"time"
	"unicode"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/messages"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
)

// selectorAll selects all the devices as targets.
var selectorAll = "all"

// nextCommandToken defines a break input.
var nextCommandToken = "next"

// inputReplacer replaces common sentence delimiters with nextCommandToken.
var inputReplacer = strings.NewReplacer(
	".", " "+nextCommandToken+" ",
	";", " "+nextCommandToken+" ",
	"then", nextCommandToken,
)

// CommandParser converts free-form user text into executable commands.
type CommandParser struct {
	selectors       map[string][]*device.Device
	selectorsLabels map[string]string
}

// Command represents an instruction ready for dispatch.
type Command struct {
	Targets []device.Serial
	Msgs    []*protocol.Message
}

// selectorMatch represents a successful match between a phrase in the input
// and one or more known devices. It stores the matched phrase span and the
// resolved devices so later stages can convert them into targets.
type selectorMatch struct {
	Match   string
	Span    int
	Devices []*device.Device
}

// NewCommandParser builds a parser instance from a device list.
// It precomputes selector phrases so parsing later inputs is fast.
func NewCommandParser(devices []device.Device) *CommandParser {
	c := &CommandParser{}
	c.selectorsFromDevices(devices)
	return c
}

// Parse converts raw user input into a list of executable Commands.
func (p *CommandParser) Parse(input string) []Command {
	words := p.tokenize(input)
	selectorMatches := p.matchEntities(words)
	tokens := p.classifyTokens(words, selectorMatches)
	intentAtoms := p.buildIntentAtoms(tokens)
	intents := p.buildIntent(intentAtoms)
	return p.buildCommands(intents)
}

// ForEachSend calls sender for every message/target pair in the command.
// It does not handle errors, retries, or concurrency — callers are expected
// to implement their own sending strategy.
func (c Command) ForEachSend(sender func(device.Serial, *protocol.Message)) {
	for _, msg := range c.Msgs {
		for _, s := range c.Targets {
			sender(s, msg)
		}
	}
}

// tokenize normalizes the input string into a slice of lowercase words.
// It removes punctuation, splits on non-alphanumeric characters, and
// replaces common separators (like "then", ",", etc.) with command breaks.
func (p *CommandParser) tokenize(input string) []string {
	// Replace common terminating tokens with nextCommandToken.
	input = inputReplacer.Replace(input)

	// Clean up any non alphanumeric character, including leading and trailing punctuation.
	f := func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	}

	// Split input into words
	return strings.FieldsFunc(strings.ToLower(input), f)
}

// matchEntities scans the token stream and finds the longest selector phrase
// starting at each position. When a selector is found, it records the span and
// associated devices so later stages can treat it as a single semantic unit.
func (p *CommandParser) matchEntities(words []string) map[int]*selectorMatch {
	matches := make(map[int]*selectorMatch)
	for i := 0; i < len(words); i++ {
		for j := len(words); j > i; j-- {
			phrase := strings.Join(words[i:j], " ")
			if devices, ok := p.selectors[phrase]; ok {
				matches[i] = &selectorMatch{Match: phrase, Span: j - i, Devices: devices}
				i = j - 1
				break
			}
		}
	}
	return matches
}

// buildCommands converts resolved intents into executable Command values.
// Each Command bundles protocol messages together with the target device serials.
func (p *CommandParser) buildCommands(intents []intent) []Command {
	var cmds []Command
	for _, c := range intents {
		if c.Action == nil || len(c.Targets) == 0 {
			continue
		}

		a := c.Action
		cmd := Command{Targets: dedupeSerials(c.Targets)}
		if a.Brightness != nil || a.Hue != nil || a.Saturation != nil || a.Kelvin != nil {
			d := time.Millisecond
			if a.Duration != nil {
				d = *a.Duration
			}
			cmd.Msgs = append(cmd.Msgs, messages.SetColor(a.Hue, a.Saturation, a.Brightness, a.Kelvin, d, 0))
		}
		if a.Power != nil {
			if *a.Power {
				cmd.Msgs = append(cmd.Msgs, messages.SetPowerOn())
			} else {
				cmd.Msgs = append(cmd.Msgs, messages.SetPowerOff())
			}
		}

		cmds = append(cmds, cmd)
	}

	return cmds
}

// selectorsFromDevices parses a list of device.Device and sets a mapping that
// groups devices by serial (hex string), group name, location name and "all" selectors.
// It also builds a map of lowercased to original labels for device names, groups and locations.
// Note: Selectors are lowercased for ease and performance, so a single selectors might
// include multiple devices, if labels contain the same letters. selectorsLabels are also
// affected by this edge case and will only match the latest device sharing the label.
func (p *CommandParser) selectorsFromDevices(devices []device.Device) {
	p.selectors = make(map[string][]*device.Device)
	p.selectorsLabels = make(map[string]string)

	for i := range devices {
		d := &devices[i]
		serial, label := d.Serial.String(), strings.ToLower(d.Label)
		group, location := strings.ToLower(d.Group), strings.ToLower(d.Location)

		p.selectors[serial] = append(p.selectors[serial], d)

		p.selectors[label] = append(p.selectors[label], d)
		p.selectorsLabels[label] = d.Label

		if group != "" {
			p.selectors[group] = append(p.selectors[group], d)
			p.selectorsLabels[group] = d.Group
		}
		if location != "" {
			p.selectors[location] = append(p.selectors[location], d)
			p.selectorsLabels[location] = d.Location
		}

		p.selectors[selectorAll] = append(p.selectors[selectorAll], d)
	}
}

// originalForSelector tries to find the original label for a selector, otherwise
// it returns the selector.
func (p *CommandParser) originalForSelector(l string) string {
	if v, ok := p.selectorsLabels[l]; ok {
		return v
	}
	return l
}

func dedupeSerials(targets []*device.Device) []device.Serial {
	seen := make(map[device.Serial]bool)
	var serials []device.Serial
	for _, t := range targets {
		if _, ok := seen[t.Serial]; !ok {
			seen[t.Serial] = true
			serials = append(serials, t.Serial)
		}
	}
	return serials
}
