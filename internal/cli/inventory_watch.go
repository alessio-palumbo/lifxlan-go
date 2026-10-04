package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

type inventoryView struct {
	Timestamp time.Time
	Devices   []deviceView
}

// Match the monitor example's conservative terminal detection. Ordinary files,
// pipes, and TERM=dumb never receive screen-control sequences.
func terminalOutput(out io.Writer) bool {
	file, ok := out.(*os.File)
	if !ok || os.Getenv("TERM") == "dumb" {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// Poll only the local cache. This does not change discovery or state polling.
func watchDevices(ctx context.Context, c backend, out io.Writer, format string, interval time.Duration, inPlace bool) error {
	if interval <= 0 {
		return fmt.Errorf("inventory refresh interval must be positive")
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		devices := c.GetDevices()
		device.SortDevices(devices)
		if err := printInventory(out, devices, format, time.Now().UTC(), inPlace); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func printInventory(out io.Writer, devices []device.Device, format string, at time.Time, inPlace bool) error {
	if format == "json" {
		views := make([]deviceView, len(devices))
		for i, d := range devices {
			views[i] = viewDevice(d)
		}
		return json.NewEncoder(out).Encode(inventoryView{Timestamp: at, Devices: views})
	}
	var frame bytes.Buffer
	fmt.Fprintf(&frame, "%s  devices=%d  Ctrl+C to stop\n", at.Format(time.RFC3339), len(devices))
	if len(devices) == 0 {
		fmt.Fprintln(&frame, "Waiting for devices; discovery continues...")
	} else {
		if err := printDevices(&frame, devices); err != nil {
			return err
		}
	}
	if inPlace {
		if _, err := io.WriteString(out, "\x1b[H\x1b[2J"); err != nil {
			return err
		}
	}
	_, err := out.Write(frame.Bytes())
	return err
}
