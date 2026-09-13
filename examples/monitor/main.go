// Command monitor prints an inventory that follows observed LIFX device state.
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/controller"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

const renderInterval = 100 * time.Millisecond

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	ctrl, err := controller.New()
	if err != nil {
		log.Fatal(err)
	}
	defer ctrl.Close()

	devices := make(map[device.Serial]device.Device)
	events := ctrl.SubscribeDevices(ctx)
	render := time.NewTicker(renderInterval)
	defer render.Stop()
	inPlace := terminalOutput(os.Stdout)

	var lastEvent controller.DeviceEvent
	var dirty bool
	var ready bool
	for {
		select {
		case event, ok := <-events:
			if !ok {
				if dirty && ready {
					printDevices(os.Stdout, lastEvent, devices, inPlace)
				}
				return
			}
			applyEvent(ctrl, devices, event)
			lastEvent = event
			dirty = true
			if event.Type == controller.DeviceEventSnapshotComplete {
				ready = true
			}
		case <-render.C:
			if dirty && ready {
				printDevices(os.Stdout, lastEvent, devices, inPlace)
				dirty = false
			}
		}
	}
}

func applyEvent(ctrl *controller.Controller, devices map[device.Serial]device.Device, event controller.DeviceEvent) {
	switch event.Type {
	case controller.DeviceEventAdded, controller.DeviceEventUpdated:
		devices[event.Device.Serial] = event.Device
	case controller.DeviceEventRemoved:
		delete(devices, event.Device.Serial)
	case controller.DeviceEventResyncRequired:
		clear(devices)
		for _, d := range ctrl.GetDevices() {
			devices[d.Serial] = d
		}
	}
}

func terminalOutput(out *os.File) bool {
	info, err := out.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0 && os.Getenv("TERM") != "dumb"
}

func printDevices(out io.Writer, event controller.DeviceEvent, devices map[device.Serial]device.Device, inPlace bool) {
	var display bytes.Buffer
	eventName := event.Type.String()
	if event.Type == controller.DeviceEventAdded && event.Initial {
		eventName = "initial"
	}
	fmt.Fprintf(&display, "%s  event=%s  revision=%d  devices=%d\n",
		time.Now().Format(time.RFC3339), eventName, event.Revision, len(devices))

	sorted := make([]device.Device, 0, len(devices))
	for _, d := range devices {
		sorted = append(sorted, d)
	}
	device.SortDevices(sorted)

	table := tabwriter.NewWriter(&display, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "SERIAL\tNAME\tLOCATION\tGROUP\tPRODUCT ID\tREGISTRY NAME\tTYPE\tPOWER\tCOLOR\tZONES")
	for _, d := range sorted {
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\n",
			d.Serial,
			displayText(d.Label),
			displayText(d.Location),
			displayText(d.Group),
			d.ProductID,
			displayText(d.RegistryName),
			d.Type,
			powerSummary(d),
			colorSummary(d),
			zoneSummary(d),
		)
	}
	if err := table.Flush(); err != nil {
		log.Printf("flush device table: %v", err)
		return
	}
	if inPlace {
		fmt.Fprint(out, "\x1b[H\x1b[2J")
	} else {
		fmt.Fprintln(out)
	}
	fmt.Fprint(out, display.String())
}

func displayText(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func powerSummary(d device.Device) string {
	if d.Type != device.DeviceTypeSwitch {
		if d.PoweredOn {
			return "on"
		}
		return "off"
	}
	if len(d.Relays) == 0 {
		return "-"
	}

	relays := make([]string, 0, len(d.Relays))
	for _, relay := range d.Relays {
		power := "off"
		if relay.PoweredOn {
			power = "on"
		}
		relays = append(relays, fmt.Sprintf("%d:%s", relay.Index, power))
	}
	return strings.Join(relays, ",")
}

func colorSummary(d device.Device) string {
	if d.Type == device.DeviceTypeSwitch {
		return "-"
	}
	return fmt.Sprintf("h=%.1f s=%.1f b=%.1f k=%d",
		d.Color.Hue, d.Color.Saturation, d.Color.Brightness, d.Color.Kelvin)
}

func zoneSummary(d device.Device) string {
	switch d.LightType {
	case device.LightTypeMultiZone:
		return fmt.Sprintf("%d", len(d.MultizoneProperties.Zones))
	case device.LightTypeMatrix:
		return fmt.Sprintf("%d", matrixZoneCount(d))
	default:
		return "-"
	}
}

func matrixZoneCount(d device.Device) int {
	var zones int
	for _, colors := range d.MatrixProperties.ChainZones {
		zones += len(colors)
	}
	return zones
}
