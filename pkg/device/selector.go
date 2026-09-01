package device

import (
	"fmt"
	"strings"
)

const (
	// SelectorAll matches every device in a selector list.
	SelectorAll = "all"

	selectorSerial     = "serial"
	selectorLabel      = "label"
	selectorGroup      = "group"
	selectorLocation   = "location"
	selectorGroupID    = "group_id"
	selectorLocationID = "location_id"
)

type selectorKind uint8

const (
	selectorBare selectorKind = iota
	selectorAll
	selectorBySerial
	selectorByLabel
	selectorByGroup
	selectorByLocation
	selectorByGroupID
	selectorByLocationID
)

type parsedSelector struct {
	kind       selectorKind
	value      string
	serial     Serial
	groupID    GroupID
	locationID LocationID
}

// SplitSelectors splits a comma-separated selector string and removes empty
// selectors.
func SplitSelectors(input string) []string {
	parts := strings.Split(input, ",")
	selectors := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			selectors = append(selectors, part)
		}
	}
	return selectors
}

// MatchSelector returns devices matched by selector.
//
// Selectors are case-insensitive exact matches against "all", serial, label,
// group, or location. Results preserve the order of devices and are de-duplicated.
func MatchSelector(selector string, devices []Device) []Device {
	parsed, err := parseSelector(selector, false)
	if err != nil {
		return nil
	}
	if parsed.kind == selectorAll {
		return append([]Device(nil), devices...)
	}

	var matches []Device
	seen := make(map[Serial]bool)
	for _, d := range devices {
		matched := selectorMatchesDevice(parsed, d)
		// Preserve the original bare-selector meaning for labels that contain a
		// now-recognized typed prefix. Strict API callers use ResolveSelectorList.
		if !matched && parsed.kind != selectorBare {
			matched = selectorMatchesDevice(parsedSelector{kind: selectorBare, value: normalizeSelector(selector)}, d)
		}
		if matched && !seen[d.Serial] {
			seen[d.Serial] = true
			matches = append(matches, d)
		}
	}
	return matches
}

// ResolveSelectorList resolves selector tokens to devices.
//
// Unlike ResolveSelectors, tokens containing a colon use the strict typed
// selector grammar and unknown selector types return an error. Results preserve
// selector order and device order within each selector, and are de-duplicated.
func ResolveSelectorList(selectors []string, devices []Device) ([]Device, error) {
	var resolved []Device
	seen := make(map[Serial]bool)
	for _, selector := range selectors {
		parsed, err := parseSelector(selector, true)
		if err != nil {
			return nil, fmt.Errorf("invalid selector %q: %w", selector, err)
		}
		for _, d := range devices {
			if seen[d.Serial] || !selectorMatchesDevice(parsed, d) {
				continue
			}
			seen[d.Serial] = true
			resolved = append(resolved, d)
		}
	}
	return resolved, nil
}

// ResolveSelectors resolves a comma-separated selector string to devices.
//
// Each selector is matched independently and combined in selector order. Duplicate
// serials are removed while preserving the first match.
func ResolveSelectors(input string, devices []Device) []Device {
	var resolved []Device
	seen := make(map[Serial]bool)
	for _, selector := range SplitSelectors(input) {
		for _, d := range MatchSelector(selector, devices) {
			if seen[d.Serial] {
				continue
			}
			seen[d.Serial] = true
			resolved = append(resolved, d)
		}
	}
	return resolved
}

// ResolveSelectorSerials resolves a comma-separated selector string to serials.
func ResolveSelectorSerials(input string, devices []Device) []Serial {
	matches := ResolveSelectors(input, devices)
	if len(matches) == 0 {
		return nil
	}
	serials := make([]Serial, len(matches))
	for i, d := range matches {
		serials[i] = d.Serial
	}
	return serials
}

func selectorMatchesDevice(selector parsedSelector, d Device) bool {
	switch selector.kind {
	case selectorAll:
		return true
	case selectorBySerial:
		return d.Serial == selector.serial
	case selectorByLabel:
		return normalizeSelector(d.Label) == selector.value
	case selectorByGroup:
		return normalizeSelector(d.Group) == selector.value
	case selectorByLocation:
		return normalizeSelector(d.Location) == selector.value
	case selectorByGroupID:
		return d.GroupID == selector.groupID
	case selectorByLocationID:
		return d.LocationID == selector.locationID
	case selectorBare:
		if d.Serial == selector.serial && !selector.serial.IsNil() {
			return true
		}
		return normalizeSelector(d.Label) == selector.value ||
			normalizeSelector(d.Group) == selector.value ||
			normalizeSelector(d.Location) == selector.value
	default:
		return false
	}
}

func parseSelector(selector string, strict bool) (parsedSelector, error) {
	key := normalizeSelector(selector)
	if key == "" {
		return parsedSelector{}, fmt.Errorf("selector is empty")
	}
	if key == SelectorAll {
		return parsedSelector{kind: selectorAll}, nil
	}

	prefix, value, typed := strings.Cut(key, ":")
	if typed {
		value = strings.TrimSpace(value)
		if value == "" {
			return parsedSelector{}, fmt.Errorf("%s selector value is empty", prefix)
		}
		switch prefix {
		case selectorSerial:
			serial, err := SerialFromHex(value)
			if err != nil {
				return parsedSelector{}, fmt.Errorf("parse serial: %w", err)
			}
			return parsedSelector{kind: selectorBySerial, serial: serial}, nil
		case selectorLabel:
			return parsedSelector{kind: selectorByLabel, value: value}, nil
		case selectorGroup:
			return parsedSelector{kind: selectorByGroup, value: value}, nil
		case selectorLocation:
			return parsedSelector{kind: selectorByLocation, value: value}, nil
		case selectorGroupID:
			id, err := ParseGroupID(value)
			if err != nil {
				return parsedSelector{}, err
			}
			return parsedSelector{kind: selectorByGroupID, groupID: id}, nil
		case selectorLocationID:
			id, err := ParseLocationID(value)
			if err != nil {
				return parsedSelector{}, err
			}
			return parsedSelector{kind: selectorByLocationID, locationID: id}, nil
		default:
			if strict {
				return parsedSelector{}, fmt.Errorf("unknown selector type %q", prefix)
			}
		}
	}

	parsed := parsedSelector{kind: selectorBare, value: key}
	if serial, err := SerialFromHex(key); err == nil {
		parsed.serial = serial
	}
	return parsed, nil
}

func normalizeSelector(selector string) string {
	return strings.ToLower(strings.TrimSpace(selector))
}
