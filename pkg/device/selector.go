package device

import "strings"

const (
	// SelectorAll matches every device in a selector list.
	SelectorAll = "all"
)

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
	key := normalizeSelector(selector)
	if key == "" {
		return nil
	}
	if key == SelectorAll {
		return append([]Device(nil), devices...)
	}
	if serial, err := SerialFromHex(key); err == nil {
		for _, d := range devices {
			if d.Serial == serial {
				return []Device{d}
			}
		}
		return nil
	}

	var matches []Device
	seen := make(map[Serial]bool)
	for _, d := range devices {
		if selectorMatchesDevice(key, d) && !seen[d.Serial] {
			seen[d.Serial] = true
			matches = append(matches, d)
		}
	}
	return matches
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

func selectorMatchesDevice(selector string, d Device) bool {
	return normalizeSelector(d.Label) == selector ||
		normalizeSelector(d.Group) == selector ||
		normalizeSelector(d.Location) == selector
}

func normalizeSelector(selector string) string {
	return strings.ToLower(strings.TrimSpace(selector))
}
