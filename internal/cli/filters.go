package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

type listFilters map[string][]string

func parseFilters(values []string) (listFilters, error) {
	filters := make(listFilters)
	for _, value := range values {
		key, text, ok := strings.Cut(value, "=")
		key = strings.ToLower(strings.TrimSpace(key))
		text = strings.TrimSpace(text)
		if !ok || text == "" {
			return nil, fmt.Errorf("invalid filter %q: expected key=value with a nonempty value", value)
		}
		switch key {
		case "location", "group", "label":
		case "serial":
			serial, err := device.SerialFromHex(text)
			if err != nil {
				return nil, fmt.Errorf("invalid serial %q: expected 12 hexadecimal digits", text)
			}
			text = serial.String()
		case "product_id":
			id, err := strconv.ParseUint(text, 10, 32)
			if err != nil || id == 0 {
				return nil, fmt.Errorf("invalid product_id %q: expected a positive uint32", text)
			}
			text = strconv.FormatUint(id, 10)
		case "firmware_version":
			major, minor, ok := strings.Cut(text, ".")
			_, majorErr := strconv.ParseUint(major, 10, 16)
			_, minorErr := strconv.ParseUint(minor, 10, 16)
			if !ok || majorErr != nil || minorErr != nil {
				return nil, fmt.Errorf("invalid firmware_version %q: expected major.minor", text)
			}
		case "type":
			text = strings.ToLower(text)
			switch text {
			case "single_zone", "multi_zone", "matrix", "switch":
			default:
				return nil, fmt.Errorf("invalid type %q: use single_zone, multi_zone, matrix, or switch", text)
			}
		default:
			return nil, fmt.Errorf("unknown filter key %q: use serial, label, location, group, product_id, firmware_version, or type", key)
		}
		filters[key] = append(filters[key], text)
	}
	return filters, nil
}

func (filters listFilters) matches(d device.Device) bool {
	for key, values := range filters {
		var actual string
		switch key {
		case "serial":
			actual = d.Serial.String()
		case "label":
			actual = d.Label
		case "location":
			actual = d.Location
		case "group":
			actual = d.Group
		case "firmware_version":
			actual = d.FirmwareVersion
		case "product_id":
			if d.ProductID != 0 {
				actual = strconv.FormatUint(uint64(d.ProductID), 10)
			}
		case "type":
			if d.ProductID != 0 {
				actual = kind(d)
			}
		}
		if actual == "" {
			return false
		}
		matched := false
		for _, value := range values {
			if strings.EqualFold(value, actual) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func (filters listFilters) apply(devices []device.Device) []device.Device {
	if len(filters) == 0 {
		return devices
	}
	selected := make([]device.Device, 0, len(devices))
	for _, d := range devices {
		if filters.matches(d) {
			selected = append(selected, d)
		}
	}
	return selected
}
