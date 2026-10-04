package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/controller"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

// These are categories from the controller, not per-field diffs or packet types.
var changeCategories = []struct {
	mask controller.DeviceChange
	name string
}{
	{controller.DeviceChangeLabel, "label"}, {controller.DeviceChangeProduct, "product"},
	{controller.DeviceChangeFirmware, "firmware"}, {controller.DeviceChangeLocation, "location"},
	{controller.DeviceChangeGroup, "group"}, {controller.DeviceChangeWiFi, "wifi"},
	{controller.DeviceChangeLight, "light"}, {controller.DeviceChangeMatrix, "matrix"},
	{controller.DeviceChangeMultizone, "multizone"}, {controller.DeviceChangeButtons, "buttons"},
	{controller.DeviceChangeButtonConfig, "button_config"}, {controller.DeviceChangeRelays, "relays"},
	{controller.DeviceChangeEffect, "effect"}, {controller.DeviceChangeUptime, "uptime"},
}

func printEventAt(out io.Writer, event controller.DeviceEvent, at time.Time) error {
	prefix := fmt.Sprintf("%s revision %-6d", at.Format("2006-01-02T15:04:05.000Z07:00"), event.Revision)
	switch event.Type {
	case controller.DeviceEventSnapshotComplete:
		_, err := fmt.Fprintf(out, "%s snapshot_complete  initial inventory received (state may still be incomplete)\n", prefix)
		return err
	case controller.DeviceEventResyncRequired:
		_, err := fmt.Fprintf(out, "%s resync_required    events dropped; use devices list for the current inventory\n", prefix)
		return err
	}
	name := event.Type.String()
	if event.Type == controller.DeviceEventAdded && event.Initial {
		name = "initial"
	}
	d := event.Device
	var fields []string
	if event.Type == controller.DeviceEventUpdated {
		var names []string
		remaining := event.Changes
		for _, category := range changeCategories {
			if event.Changes.Has(category.mask) {
				names = append(names, category.name)
				remaining &^= category.mask
			}
		}
		if remaining != 0 {
			names = append(names, fmt.Sprintf("unknown(0x%x)", uint64(remaining)))
		}
		if len(names) == 0 {
			names = append(names, "unspecified")
		}
		fields = append(fields, "changes="+strings.Join(names, ","))
		fields = append(fields, eventValues(d, event.Changes)...)
	} else if event.Type == controller.DeviceEventAdded {
		fields = append(fields, "cached:", "type="+kind(d), "zones="+zones(d), "power="+power(d))
		if hasLight(d) {
			fields = append(fields, "color="+eventColor(d.Color))
		}
		fields = append(fields, "firmware="+firmware(d), "ip="+ipAddress(d))
	} else if event.Type == controller.DeviceEventRemoved {
		fields = append(fields, "session ended (liveness timeout or removal)")
	}
	_, err := fmt.Fprintf(out, "%s %-8s %s %q %s\n", prefix, name, d.Serial, safeText(d.Label), strings.Join(fields, " "))
	return err
}

func eventValues(d device.Device, changes controller.DeviceChange) []string {
	var values []string
	if changes.Has(controller.DeviceChangeLabel) {
		values = append(values, fmt.Sprintf("label=%q", safeText(d.Label)))
	}
	if changes.Has(controller.DeviceChangeProduct) {
		values = append(values, fmt.Sprintf("product_id=%d type=%s zones=%s", d.ProductID, kind(d), zones(d)))
	}
	if changes.Has(controller.DeviceChangeFirmware) {
		values = append(values, "firmware="+firmware(d))
	}
	if changes.Has(controller.DeviceChangeLocation) {
		values = append(values, fmt.Sprintf("location=%q", safeText(d.Location)))
	}
	if changes.Has(controller.DeviceChangeGroup) {
		values = append(values, fmt.Sprintf("group=%q", safeText(d.Group)))
	}
	if changes.Has(controller.DeviceChangeWiFi) {
		values = append(values, "rssi/snr="+wifiSignal(d))
	}
	if changes.Has(controller.DeviceChangeLight) {
		values = append(values, "power="+power(d))
		if hasLight(d) {
			values = append(values, "color="+eventColor(d.Color))
		}
	}
	if changes.Has(controller.DeviceChangeMatrix) {
		m := d.MatrixProperties
		values = append(values, fmt.Sprintf("zones=%s geometry=%dx%d chains=%d", zones(d), m.Width, m.Height, m.ChainLength), cachedColorSummary(m.ChainZones))
	}
	if changes.Has(controller.DeviceChangeMultizone) {
		values = append(values, "zones="+zones(d), cachedColorSummary([][]packets.LightHsbk{d.MultizoneProperties.Zones}))
	}
	if changes.Has(controller.DeviceChangeButtons) {
		values = append(values, fmt.Sprintf("buttons=%+v", d.Buttons))
	}
	if changes.Has(controller.DeviceChangeButtonConfig) {
		values = append(values, fmt.Sprintf("haptic=%dms backlight_on=%s backlight_off=%s", d.ButtonConfig.HapticDurationMs, eventColor(d.ButtonConfig.BacklightOnColor), eventColor(d.ButtonConfig.BacklightOffColor)))
	}
	if changes.Has(controller.DeviceChangeRelays) {
		var relays []string
		for _, relay := range d.Relays {
			state := "off"
			if relay.PoweredOn {
				state = "on"
			}
			relays = append(relays, fmt.Sprintf("%d:%s", relay.Index, state))
		}
		values = append(values, "relays=["+strings.Join(relays, ",")+"]")
	}
	if changes.Has(controller.DeviceChangeEffect) {
		switch d.LightType {
		case device.LightTypeMatrix:
			values = append(values, "effect="+d.MatrixProperties.Effect.Type.String())
		case device.LightTypeMultiZone:
			values = append(values, "effect="+d.MultizoneProperties.Effect.Type.String())
		}
	}
	if changes.Has(controller.DeviceChangeUptime) {
		values = append(values, "uptime="+estimatedUptime(d))
		if !d.EstimatedBootedAt.IsZero() {
			values = append(values, "estimated_boot="+d.EstimatedBootedAt.Format(time.RFC3339))
		}
	}
	return values
}

func eventColor(color device.Color) string {
	return fmt.Sprintf("HSBK(%.1f,%.1f,%.1f,%d)", color.Hue, color.Saturation, color.Brightness, color.Kelvin)
}

// Bound the preview instead of dumping hundreds of zones per event. Buffer
// contents may be partial; this makes no receipt-completeness assertion.
func cachedColorSummary(chains [][]packets.LightHsbk) string {
	var preview []string
	count := 0
	min, max := 100.0, 0.0
	for _, colors := range chains {
		for _, raw := range colors {
			color := device.NewColor(raw)
			count++
			if color.Brightness < min {
				min = color.Brightness
			}
			if color.Brightness > max {
				max = color.Brightness
			}
			if len(preview) < 3 {
				preview = append(preview, eventColor(color))
			}
		}
	}
	if count == 0 {
		return "cached_colors=0"
	}
	if count > len(preview) {
		preview = append(preview, fmt.Sprintf("... +%d", count-len(preview)))
	}
	return fmt.Sprintf("cached_colors=%d brightness=%.1f..%.1f%% sample=[%s]", count, min, max, strings.Join(preview, " "))
}
