// Package cli implements the direct-LAN diagnostic CLI without terminal dependencies.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/command"
	"github.com/alessio-palumbo/lifxlan-go/pkg/controller"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/effects"
	"github.com/alessio-palumbo/lifxlan-go/pkg/effects/adapters"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
)

const help = `lifxlan: direct LAN diagnostics (JSON output)

Usage: lifxlan COMMAND [flags] [arguments]
  devices                         list discovered devices
  inspect --target LABEL|SERIAL   inspect one device, including estimated uptime
  ping --target LABEL|SERIAL       sequential echo samples
  watch                           stream device events as JSON lines
  command [--dry-run] "TEXT"       compile and optionally send a natural-language command
  snapshot --target LABEL|SERIAL   capture complete observed state to stdout
  effects list                    list effects and configurable parameters (offline)
  effects run [flags] EFFECT_ID    run an effect on one light

Common flags (before positional arguments):
  --discover-for 3s   discovery window; inventory is not a state-readiness guarantee
  --timeout 3s        timeout for each ping or snapshot capture
  --target VALUE      exact label or 12-digit serial; no implicit all-device target
  --verbose           controller diagnostic logging to stderr
Ping: --count 5
Snapshot: --fresh
Effects run: --config FILE --step 100ms --duration 0s --restore=true
  Config is an effects.Config JSON object. Duration zero runs until Ctrl+C.
  Effects preserve power; restoration uses a fresh snapshot and a separate timeout.
Dry-run sends no control messages, but discovery still uses LAN traffic.
`

type backend interface {
	Close() error
	GetDevices() []device.Device
	Send(device.Serial, *protocol.Message) error
	Ping(context.Context, device.Serial) (time.Duration, error)
	SubscribeDevices(context.Context, ...controller.SubscriptionOption) <-chan controller.DeviceEvent
	CaptureStateSnapshot(context.Context, []device.Serial, controller.SnapshotOptions) (device.StateSnapshot, error)
	RestoreStateSnapshot(context.Context, device.StateSnapshot, controller.RestoreOptions) error
}

type factory func(bool, io.Writer) (backend, error)

// Keep the library's rich device representation but make serials copyable.
type deviceView struct {
	device.Device
	Serial string
}

type packetView struct {
	Type    uint16
	Name    string
	Payload any
}

type commandView struct {
	Targets  []string
	Messages []packetView
}

// Run executes one command. Help and effect listing never open a LAN socket.
func Run(ctx context.Context, args []string, out, diagnostic io.Writer) error {
	return run(ctx, args, out, diagnostic, func(verbose bool, w io.Writer) (backend, error) {
		var opts []controller.Option
		if verbose {
			opts = append(opts, controller.WithLogger(slog.New(slog.NewTextHandler(w, nil))))
		}
		return controller.New(opts...)
	})
}

func run(ctx context.Context, args []string, out, diagnostic io.Writer, create factory) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		_, err := io.WriteString(out, help)
		return err
	}
	name := args[0]
	args = args[1:]
	if name == "effects" {
		if len(args) == 0 {
			return errors.New("expected effects list or effects run")
		}
		name += " " + args[0]
		args = args[1:]
	}
	switch name {
	case "devices", "inspect", "ping", "watch", "command", "snapshot", "effects list", "effects run":
	default:
		return fmt.Errorf("unknown command %q; use lifxlan help", name)
	}
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	f.SetOutput(diagnostic)
	discovery := f.Duration("discover-for", 3*time.Second, "discovery window")
	timeout := f.Duration("timeout", 3*time.Second, "operation timeout")
	target := f.String("target", "", "exact label or serial")
	verbose := f.Bool("verbose", false, "controller logging")
	var count int
	var dry, fresh, restore bool
	var step, duration time.Duration
	var configPath string
	switch name {
	case "ping":
		f.IntVar(&count, "count", 5, "sequential samples")
	case "command":
		f.BoolVar(&dry, "dry-run", false, "print without sending control messages")
	case "snapshot":
		f.BoolVar(&fresh, "fresh", false, "require observations after capture starts")
	case "effects run":
		f.StringVar(&configPath, "config", "", "effect config JSON file")
		f.DurationVar(&step, "step", 100*time.Millisecond, "frame interval")
		f.DurationVar(&duration, "duration", 0, "run duration; zero until interrupted")
		f.BoolVar(&restore, "restore", true, "restore fresh captured state on exit")
	}
	if err := f.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, err = io.WriteString(out, help)
		}
		return err
	}
	if *discovery <= 0 || *timeout <= 0 {
		return errors.New("discovery window and timeout must be positive")
	}
	if name == "command" || name == "effects run" {
		if f.NArg() != 1 {
			return fmt.Errorf("%s requires exactly one argument; put flags before it", name)
		}
	} else if f.NArg() != 0 {
		return errors.New("unexpected arguments; put flags before positional arguments")
	}
	if name == "ping" && count <= 0 {
		return errors.New("count must be positive")
	}
	if name == "effects run" && (step < 20*time.Millisecond || duration < 0) {
		return errors.New("step must be at least 20ms and duration nonnegative")
	}
	if name == "inspect" || name == "ping" || name == "snapshot" || name == "effects run" {
		if *target == "" {
			return errors.New("an explicit --target label or serial is required")
		}
	}
	if name == "effects list" {
		var defs []any
		for _, d := range effects.Definitions() {
			defs = append(defs, struct {
				ID                 effects.EffectID
				Label, Description string
				DeviceKinds        []device.LightType
				Params             []effects.ParamDefinition
			}{d.ID, d.Label, d.Description, d.DeviceKinds, d.Params})
		}
		return writeJSON(out, defs)
	}
	config := effects.Config{}
	if name == "effects run" {
		config.ID = effects.EffectID(f.Arg(0))
		if configPath != "" {
			file, err := os.Open(configPath)
			if err != nil {
				return err
			}
			defer file.Close()
			dec := json.NewDecoder(file)
			dec.DisallowUnknownFields()
			if err := dec.Decode(&config); err != nil {
				return fmt.Errorf("effect config: %w", err)
			}
			var extra any
			if err := dec.Decode(&extra); err != io.EOF {
				return errors.New("effect config must contain exactly one JSON object")
			}
			if config.ID != effects.EffectID(f.Arg(0)) {
				return errors.New("config ID must match the effect argument")
			}
		}
		if _, ok := effects.Definition(config.ID); !ok {
			return fmt.Errorf("unknown effect %q", config.ID)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	c, err := create(*verbose, diagnostic)
	if err != nil {
		return err
	}
	defer c.Close()
	if name == "watch" {
		enc := json.NewEncoder(out)
		events := c.SubscribeDevices(ctx)
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case event, ok := <-events:
				if !ok {
					return nil
				}
				if err := enc.Encode(event); err != nil {
					return err
				}
			}
		}
	}
	if err := wait(ctx, *discovery); err != nil {
		return err
	}
	devices := c.GetDevices()
	device.SortDevices(devices)
	if name == "devices" {
		views := make([]deviceView, len(devices))
		for i, d := range devices {
			views[i] = deviceView{Device: d, Serial: d.Serial.String()}
		}
		return writeJSON(out, views)
	}
	if name == "command" {
		return executeCommand(ctx, c, devices, f.Arg(0), dry, out)
	}
	d, err := selectDevice(devices, *target)
	if err != nil {
		return err
	}
	switch name {
	case "inspect":
		uptime, known := d.Uptime()
		return writeJSON(out, struct {
			Device      deviceView
			Uptime      string
			UptimeKnown bool
		}{deviceView{Device: d, Serial: d.Serial.String()}, uptime.String(), known})
	case "ping":
		var failures []error
		for i := 0; i < count; i++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			probe, cancel := context.WithTimeout(ctx, *timeout)
			rtt, err := c.Ping(probe, d.Serial)
			cancel()
			result := struct {
				Serial string
				Sample int
				RTT    string
				Error  string
			}{Serial: d.Serial.String(), Sample: i + 1}
			if err != nil {
				result.Error = err.Error()
				failures = append(failures, err)
			} else {
				result.RTT = rtt.String()
			}
			if err := json.NewEncoder(out).Encode(result); err != nil {
				return err
			}
		}
		return errors.Join(failures...)
	case "snapshot":
		if d.Type != device.DeviceTypeLight {
			return errors.New("snapshot target must be a light")
		}
		snapshot, err := c.CaptureStateSnapshot(ctx, []device.Serial{d.Serial}, controller.SnapshotOptions{Timeout: *timeout, RequireFresh: fresh})
		if err != nil {
			return err
		}
		return writeJSON(out, snapshot)
	case "effects run":
		return runEffect(ctx, c, d, config, step, duration, *timeout, restore)
	}
	return nil
}

func selectDevice(devices []device.Device, target string) (device.Device, error) {
	// A serial takes precedence over labels to keep targeting unambiguous.
	if serial, err := device.SerialFromHex(target); err == nil {
		for _, d := range devices {
			if d.Serial == serial {
				return d, nil
			}
		}
		return device.Device{}, fmt.Errorf("serial %s not discovered; try a longer --discover-for", target)
	}
	var matches []device.Device
	for _, d := range devices {
		if strings.EqualFold(d.Label, target) {
			matches = append(matches, d)
		}
	}
	if len(matches) != 1 {
		return device.Device{}, fmt.Errorf("target %q matched %d devices; use a serial or a longer discovery window", target, len(matches))
	}
	return matches[0], nil
}

func executeCommand(ctx context.Context, c backend, devices []device.Device, text string, dry bool, out io.Writer) error {
	commands := command.NewCommandParser(devices).Parse(text)
	if len(commands) == 0 {
		return errors.New("command produced no messages or targets")
	}
	// Print the entire plan before dispatch; a failed output never triggers sends.
	plan := make([]commandView, len(commands))
	for i, cmd := range commands {
		for _, serial := range cmd.Targets {
			plan[i].Targets = append(plan[i].Targets, serial.String())
		}
		for _, msg := range cmd.Msgs {
			plan[i].Messages = append(plan[i].Messages, packetView{Type: msg.Type(), Name: fmt.Sprintf("%T", msg.Payload), Payload: msg.Payload})
		}
	}
	if err := writeJSON(out, plan); err != nil {
		return err
	}
	if dry {
		return nil
	}
	for _, cmd := range commands {
		for _, msg := range cmd.Msgs {
			for _, serial := range cmd.Targets {
				if err := ctx.Err(); err != nil {
					return err
				}
				if err := c.Send(serial, msg); err != nil {
					return fmt.Errorf("send to %s: %w", serial, err)
				}
			}
		}
	}
	return nil
}

func runEffect(ctx context.Context, c backend, d device.Device, config effects.Config, step, duration, timeout time.Duration, restore bool) (err error) {
	if d.Type != device.DeviceTypeLight {
		return errors.New("effect target must be a light")
	}
	// Observe capabilities as well as colors before constructing a renderer.
	snapshot, err := c.CaptureStateSnapshot(ctx, []device.Serial{d.Serial}, controller.SnapshotOptions{Timeout: timeout, RequireFresh: true})
	if err != nil {
		return fmt.Errorf("capture before effect: %w", err)
	}
	for _, current := range c.GetDevices() {
		if current.Serial == d.Serial {
			d = current
			break
		}
	}
	effect, err := effects.New(config, effects.CapabilitiesFromDevice(d))
	if err != nil {
		return err
	}
	if restore {
		defer func() {
			recovery, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			if restoreErr := c.RestoreStateSnapshot(recovery, snapshot, controller.RestoreOptions{}); restoreErr != nil {
				// Do not let cancellation hide a restoration failure at the CLI boundary.
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					err = nil
				}
				err = errors.Join(err, fmt.Errorf("restore after effect: %w", restoreErr))
			}
		}()
	}
	runCtx := ctx
	if duration > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, duration)
		defer cancel()
	}
	renderer := adapters.NewRendererForDevice(d, func(msg *protocol.Message) error { return c.Send(d.Serial, msg) })
	err = effects.NewRunner(effect, renderer, step).Run(runCtx)
	if duration > 0 && errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
		return nil
	}
	return err
}

func writeJSON(out io.Writer, value any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func wait(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
