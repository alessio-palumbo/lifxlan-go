package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/controller"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

// Serial-only selectors and all are immediate. Name selectors resolve once after
// discovery, then remain bound to those serials even if devices are renamed.
func streamDevices(ctx context.Context, c backend, out io.Writer, format, target string, discovery time.Duration) error {
	var selected map[device.Serial]bool
	if target != "" {
		tokens := device.SplitSelectors(target)
		if len(tokens) == 0 {
			return fmt.Errorf("target selector is empty")
		}
		if _, err := device.ResolveSelectorList(tokens, nil); err != nil {
			return err
		}
		immediate := true
		all := false
		selected = make(map[device.Serial]bool)
		for _, token := range tokens {
			normalized := strings.ToLower(strings.TrimSpace(token))
			if normalized == device.SelectorAll {
				all = true
				continue
			}
			value := strings.TrimSpace(strings.TrimPrefix(normalized, "serial:"))
			serial, err := device.SerialFromHex(value)
			if err != nil {
				immediate = false
				continue
			}
			selected[serial] = true
		}
		if !immediate {
			if err := wait(ctx, discovery); err != nil {
				return err
			}
			devices, err := resolveTargets([]string{target}, c.GetDevices(), false)
			if err != nil {
				return err
			}
			selected = make(map[device.Serial]bool)
			for _, d := range devices {
				selected[d.Serial] = true
			}
		}
		if all {
			selected = nil
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	events := c.SubscribeDevices(ctx)
	enc := json.NewEncoder(out)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-events:
			if !ok {
				return nil
			}
			if !matchesStreamTarget(event, selected) {
				continue
			}
			var err error
			if format == "json" {
				err = enc.Encode(event)
			} else {
				err = printEvent(out, event)
			}
			if err != nil {
				return err
			}
		}
	}
}

func matchesStreamTarget(event controller.DeviceEvent, serials map[device.Serial]bool) bool {
	// Control events apply to the subscription, not a device; retain them so a
	// filtered consumer can still detect initial-inventory completion or loss.
	return serials == nil || event.Type == controller.DeviceEventSnapshotComplete || event.Type == controller.DeviceEventResyncRequired || serials[event.Device.Serial]
}
