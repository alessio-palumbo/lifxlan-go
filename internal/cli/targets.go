package cli

import (
	"fmt"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

// Reuse the library grammar, but reject each unmatched token rather than
// silently applying only the matching part of a user's requested selection.
func resolveTargets(inputs []string, devices []device.Device, lightsOnly bool) ([]device.Device, error) {
	var selected []device.Device
	seen := make(map[device.Serial]bool)
	for _, input := range inputs {
		tokens := device.SplitSelectors(input)
		if len(tokens) == 0 {
			return nil, fmt.Errorf("target selector is empty")
		}
		for _, token := range tokens {
			matches, err := device.ResolveSelectorList([]string{token}, devices)
			if err != nil {
				return nil, err
			}
			if len(matches) == 0 {
				return nil, fmt.Errorf("selector %q matched no discovered devices; try a longer --discover-for", token)
			}
			eligible := 0
			for _, d := range matches {
				if lightsOnly && !hasLight(d) {
					continue
				}
				eligible++
				if !seen[d.Serial] {
					seen[d.Serial] = true
					selected = append(selected, d)
				}
			}
			if eligible == 0 {
				return nil, fmt.Errorf("selector %q matched no light-capable devices", token)
			}
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no target selectors provided")
	}
	return selected, nil
}
