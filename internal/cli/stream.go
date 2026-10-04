package cli

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/controller"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

// Serial selection is immediate. Labels resolve once after the bounded discovery
// window, then remain bound to that serial even if the device is renamed.
func streamDevices(ctx context.Context, c backend, out io.Writer, format, target string, discovery time.Duration) error {
	var selected *device.Serial
	if target != "" {
		serial, err := device.SerialFromHex(target)
		if err != nil {
			if err := wait(ctx, discovery); err != nil {
				return err
			}
			d, err := selectDevice(c.GetDevices(), target)
			if err != nil {
				return err
			}
			serial = d.Serial
		}
		selected = &serial
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

func matchesStreamTarget(event controller.DeviceEvent, serial *device.Serial) bool {
	// Control events apply to the subscription, not a device; retain them so a
	// filtered consumer can still detect initial-inventory completion or loss.
	return serial == nil || event.Type == controller.DeviceEventSnapshotComplete || event.Type == controller.DeviceEventResyncRequired || event.Device.Serial == *serial
}
