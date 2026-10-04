package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"

	"github.com/alessio-palumbo/lifxlan-go/pkg/controller"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/effects"
)

// Device labels and other network-provided text must not control the terminal.
func safeText(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value)
}

func power(d device.Device) string {
	if d.Type == device.DeviceTypeSwitch {
		return "see relays"
	}
	if d.ProductID == 0 {
		return "unknown"
	}
	if d.PoweredOn {
		return "on"
	}
	return "off"
}

func kind(d device.Device) string {
	if d.Type == device.DeviceTypeSwitch {
		return d.Type.String()
	}
	return d.LightType.String()
}

func hasLight(d device.Device) bool {
	return d.Type == device.DeviceTypeLight || d.Type == device.DeviceTypeHybrid
}

// zoneCount describes cached geometry, not observed color coverage.
// nil means either unknown geometry or a non-light device.
func zoneCount(d device.Device) *int {
	if !hasLight(d) {
		return nil
	}
	var count int
	switch d.LightType {
	case device.LightTypeSingleZone:
		if d.ProductID == 0 {
			return nil
		}
		count = 1
	case device.LightTypeMultiZone:
		count = len(d.MultizoneProperties.Zones)
	case device.LightTypeMatrix:
		m := d.MatrixProperties
		if m.Width > 0 && m.Height > 0 && m.ChainLength > 0 {
			count = m.Width * m.Height * m.ChainLength
		}
	}
	if count <= 0 {
		return nil
	}
	return &count
}

func zones(d device.Device) string {
	if !hasLight(d) {
		return "-"
	}
	if count := zoneCount(d); count != nil {
		return fmt.Sprint(*count)
	}
	return "?"
}

func ipAddress(d device.Device) string {
	if d.Address == nil || len(d.Address.IP) == 0 {
		return "-"
	}
	value := d.Address.IP.String()
	if d.Address.Zone != "" {
		value += "%" + safeText(d.Address.Zone)
	}
	return value
}

func estimatedUptime(d device.Device) string {
	if uptime, known := d.Uptime(); known {
		return uptime.Round(time.Second).String()
	}
	return "-"
}

func firmware(d device.Device) string {
	if d.FirmwareVersion == "" {
		return "-"
	}
	return safeText(d.FirmwareVersion)
}

func wifiSignal(d device.Device) string {
	// The controller also uses zero as its not-yet-observed sentinel.
	if d.WifiRSSI == 0 {
		return "-"
	}
	return fmt.Sprintf("%d", d.WifiRSSI)
}

func printDevices(out io.Writer, devices []device.Device) error {
	if len(devices) == 0 {
		_, err := fmt.Fprintln(out, "No devices discovered. Try a longer --discover-for window.")
		return err
	}
	table := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "SERIAL\tIP\tLABEL\tLOCATION\tGROUP\tTYPE\tZONES\tPRODUCT_ID\tFIRMWARE\tPOWER\tRSSI/SNR\tUPTIME")
	for _, d := range devices {
		product := "-"
		if d.ProductID != 0 {
			product = fmt.Sprint(d.ProductID)
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", d.Serial, ipAddress(d), safeText(d.Label), safeText(d.Location), safeText(d.Group), kind(d), zones(d), product, firmware(d), power(d), wifiSignal(d), estimatedUptime(d))
	}
	return table.Flush()
}

func printInspect(out io.Writer, d device.Device) error {
	table := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintf(table, "Device\t%s (%s)\nType\t%s\nProduct\t%d — %s\nGroup\t%s\nLocation\t%s\nPower (cached)\t%s\n", safeText(d.Label), d.Serial, kind(d), d.ProductID, safeText(d.RegistryName), safeText(d.Group), safeText(d.Location), power(d))
	fmt.Fprintf(table, "Firmware (cached)\t%s\nWi-Fi RSSI/SNR (cached)\t%s\n", firmware(d), wifiSignal(d))
	fmt.Fprintf(table, "Zones (total, cached)\t%s\n", zones(d))
	fmt.Fprintf(table, "IP\t%s\n", ipAddress(d))
	if uptime, known := d.Uptime(); known {
		fmt.Fprintf(table, "Estimated uptime\t%s\nEstimated boot\t%s\n", uptime.Round(time.Second), d.EstimatedBootedAt.Format(time.RFC3339))
	} else {
		fmt.Fprintln(table, "Estimated uptime\tunknown (no uptime response yet)")
	}
	if hasLight(d) {
		fmt.Fprintf(table, "Color (cached)\th=%.1f° s=%.1f%% b=%.1f%% k=%d\n", d.Color.Hue, d.Color.Saturation, d.Color.Brightness, d.Color.Kelvin)
		switch d.LightType {
		case device.LightTypeMultiZone:
			fmt.Fprintf(table, "Zones (cached)\t%d\n", len(d.MultizoneProperties.Zones))
		case device.LightTypeMatrix:
			fmt.Fprintf(table, "Matrix (cached)\t%d × %d; %d chains\n", d.MatrixProperties.Width, d.MatrixProperties.Height, d.MatrixProperties.ChainLength)
			for i, colors := range d.MatrixProperties.ChainZones {
				fmt.Fprintf(table, "Chain %d buffer\t%d pixels\n", i, len(colors))
			}
		}
	} else {
		for _, relay := range d.Relays {
			state := "off"
			if relay.PoweredOn {
				state = "on"
			}
			fmt.Fprintf(table, "Relay %d\t%s\n", relay.Index, state)
		}
	}
	fmt.Fprintln(table, "State\tcached; buffer sizes do not prove receipt completeness")
	return table.Flush()
}

func printEffects(out io.Writer, defs []effects.EffectDefinition) error {
	table := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "ID\tDEVICES\tDESCRIPTION")
	for _, d := range defs {
		kinds := make([]string, len(d.DeviceKinds))
		for i, k := range d.DeviceKinds {
			kinds[i] = k.String()
		}
		fmt.Fprintf(table, "%s\t%s\t%s\n", safeText(string(d.ID)), strings.Join(kinds, ", "), safeText(d.Description))
		params := make([]string, 0, len(d.Params))
		for _, p := range d.Params {
			params = append(params, safeText(p.Key)+"="+paramDefault(p.Default))
		}
		if len(params) > 0 {
			fmt.Fprintf(table, "\t\tDefaults: %s\n", strings.Join(params, ", "))
		}
	}
	return table.Flush()
}

func paramDefault(value any) string {
	switch v := value.(type) {
	case effects.Color:
		return fmt.Sprintf("HSBK(%.0f, %.0f, %.0f, %d)", v.Hue, v.Saturation, v.Brightness, v.Kelvin)
	case effects.Palette:
		return fmt.Sprintf("palette(%d base, %d accents, %d backgrounds)", len(v.Base), len(v.Accents), len(v.Backgrounds))
	default:
		return safeText(fmt.Sprint(value))
	}
}

func printEvent(out io.Writer, event controller.DeviceEvent) error {
	if event.Type == controller.DeviceEventSnapshotComplete {
		_, err := fmt.Fprintf(out, "revision %-6d snapshot_complete  initial inventory received (state may still be incomplete)\n", event.Revision)
		return err
	}
	if event.Type == controller.DeviceEventResyncRequired {
		_, err := fmt.Fprintf(out, "revision %-6d resync_required    events dropped; use devices for the current inventory\n", event.Revision)
		return err
	}
	_, err := fmt.Fprintf(out, "revision %-6d %-18s %s  %s  power=%s\n", event.Revision, event.Type.String(), event.Device.Serial, safeText(event.Device.Label), power(event.Device))
	return err
}

func printPlan(out io.Writer, plan []commandView, devices []device.Device, dry bool) error {
	labels := make(map[string]string, len(devices))
	for _, d := range devices {
		labels[d.Serial.String()] = safeText(d.Label)
	}
	mode := "Sending plan (delivery is not verified):"
	if dry {
		mode = "Dry-run plan (no control messages sent):"
	}
	if _, err := fmt.Fprintln(out, mode); err != nil {
		return err
	}
	for i, cmd := range plan {
		if _, err := fmt.Fprintf(out, "Command %d\n", i+1); err != nil {
			return err
		}
		for _, serial := range cmd.Targets {
			if _, err := fmt.Fprintf(out, "  Target: %s (%s)\n", labels[serial], serial); err != nil {
				return err
			}
		}
		for _, msg := range cmd.Messages {
			name := strings.TrimPrefix(msg.Name, "*packets.")
			if _, err := fmt.Fprintf(out, "  Packet: %s (type %d)\n    Payload: %+v\n", safeText(name), msg.Type, msg.Payload); err != nil {
				return err
			}
		}
	}
	return nil
}

func printPingSummary(out io.Writer, sent int, samples []time.Duration) error {
	received := len(samples)
	if _, err := fmt.Fprintf(out, "\n%d sent, %d received, %.1f%% loss\n", sent, received, 100*float64(sent-received)/float64(sent)); err != nil {
		return err
	}
	if received == 0 {
		return nil
	}
	min, max := samples[0], samples[0]
	// Average incrementally in floating point to avoid duration sum overflow.
	var avg float64
	for i, sample := range samples {
		if sample < min {
			min = sample
		}
		if sample > max {
			max = sample
		}
		avg += (float64(sample) - avg) / float64(i+1)
	}
	_, err := fmt.Fprintf(out, "RTT min/avg/max: %s / %s / %s\n", min, time.Duration(avg), max)
	return err
}
