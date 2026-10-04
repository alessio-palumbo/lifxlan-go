package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/alessio-palumbo/lifxlan-go/pkg/controller"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/effects/adapters"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxlan-go/pkg/themes"
	ucli "github.com/urfave/cli/v3"
)

func applyTheme(ctx context.Context, cmd *ucli.Command, create factory, out, diagnostic io.Writer, filters listFilters) error {
	targets := cmd.StringSlice("target")
	if (len(targets) == 0) == (len(filters) == 0) {
		return fmt.Errorf("themes apply requires either --target or --filter, not both; no implicit all-device selection")
	}
	if cmd.Duration("duration") < 0 {
		return fmt.Errorf("theme duration must be nonnegative")
	}
	file, err := os.Open(cmd.Args().Get(0))
	if err != nil {
		return err
	}
	defer file.Close()
	var theme themes.Theme
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&theme); err != nil {
		return fmt.Errorf("theme file: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("theme file must contain exactly one JSON object")
	}
	if err := theme.Validate(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	c, err := create(cmd.Bool("verbose"), diagnostic)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := wait(ctx, cmd.Duration("discover-for")); err != nil {
		return err
	}
	devices := c.GetDevices()
	var selected []device.Device
	if len(targets) > 0 {
		selected, err = resolveTargets(targets, devices, true)
		if err != nil {
			return err
		}
	} else {
		for _, d := range filters.apply(devices) {
			if hasLight(d) {
				selected = append(selected, d)
			}
		}
	}
	if len(selected) == 0 {
		return fmt.Errorf("no devices match the theme selection")
	}
	serials := make([]device.Serial, len(selected))
	for i, d := range selected {
		serials[i] = d.Serial
	}
	// Complete cached observations are sufficient for geometry. Never restore or
	// change power here; this is a persistent, one-shot color application.
	if _, err := c.CaptureStateSnapshot(ctx, serials, controller.SnapshotOptions{Timeout: cmd.Duration("timeout")}); err != nil {
		return fmt.Errorf("observe before theme: %w", err)
	}
	current := make(map[device.Serial]device.Device)
	for _, d := range c.GetDevices() {
		current[d.Serial] = d
	}
	for i, d := range selected {
		updated, ok := current[d.Serial]
		if !ok {
			return fmt.Errorf("%s disappeared before theme planning", d.Serial)
		}
		selected[i] = updated
	}
	plan, err := theme.Plan(selected, cmd.Duration("duration"))
	if err != nil {
		return err
	}
	// Compile every target's packets before the first control send. Rendering to
	// this collector uses existing orientation mapping and validates all frames.
	var packets []themePacket
	view := make([]commandView, 0, len(plan))
	for _, application := range plan {
		d := current[application.Serial]
		cmdView := commandView{Targets: []string{d.Serial.String()}}
		renderer := adapters.NewRendererForDevice(d, func(msg *protocol.Message) error {
			packets = append(packets, themePacket{Serial: d.Serial, Message: msg})
			cmdView.Messages = append(cmdView.Messages, packetView{Type: msg.Type(), Name: fmt.Sprintf("%T", msg.Payload), Payload: msg.Payload})
			return nil
		})
		if err := renderer.RenderFrame(ctx, application.Frame); err != nil {
			return fmt.Errorf("plan %s: %w", d.Serial, err)
		}
		view = append(view, cmdView)
	}
	if cmd.String("output") == "json" {
		if err := writeJSON(out, view); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(out, "Theme %q; power preserved; colors remain applied after exit.\n", safeText(theme.Name)); err != nil {
			return err
		}
		mode := "Applying"
		if cmd.Bool("dry-run") {
			mode = "Dry-run"
		}
		for i, application := range plan {
			d := current[application.Serial]
			if _, err := fmt.Fprintf(out, "%s: %s (%s), %s, zones=%s, frame=%dx%d, packets=%d, duration=%s\n", mode, safeText(d.Label), d.Serial, kind(d), zones(d), application.Frame.Width, application.Frame.Height, len(view[i].Messages), application.Frame.Duration); err != nil {
				return err
			}
			limit := min(3, len(application.Frame.Colors))
			for j := 0; j < limit; j++ {
				if _, err := fmt.Fprintf(out, "  Color %d: %s\n", j, eventColor(application.Frame.Colors[j])); err != nil {
					return err
				}
			}
			if len(application.Frame.Colors) > limit {
				if _, err := fmt.Fprintf(out, "  ... %d more logical colors (use --output json for full packets)\n", len(application.Frame.Colors)-limit); err != nil {
					return err
				}
			}
		}
	}
	if cmd.Bool("dry-run") {
		return nil
	}
	for _, packet := range packets {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := c.Send(packet.Serial, packet.Message); err != nil {
			return fmt.Errorf("theme send to %s failed; earlier sends may already have applied: %w", packet.Serial, err)
		}
	}
	return nil
}

type themePacket struct {
	Serial  device.Serial
	Message *protocol.Message
}
