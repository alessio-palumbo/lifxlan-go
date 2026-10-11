package cli

import (
	"context"
	"io"
	"time"

	ucli "github.com/urfave/cli/v3"
)

func run(ctx context.Context, args []string, out, diagnostic io.Writer, create factory) error {
	return runWithBufferReader(ctx, args, out, diagnostic, create, readMatrixBuffers)
}

func runWithBufferReader(ctx context.Context, args []string, out, diagnostic io.Writer, create factory, read bufferReader) error {
	target := func() ucli.Flag {
		return &ucli.StringFlag{Name: "target", Usage: "Selector: serial, label, group, location, all; comma-separated (device diagnostics/control and effects require one match)", Required: true}
	}
	leaf := func(name, usage, dispatchName string, flags ...ucli.Flag) *ucli.Command {
		command := &ucli.Command{Name: name, Usage: usage, Flags: flags, DisableSliceFlagSeparator: true, Action: func(ctx context.Context, cmd *ucli.Command) error {
			return dispatch(ctx, dispatchName, cmd, out, diagnostic, create, read)
		}}
		if dispatchName == "effects run" {
			command.ArgsUsage = "EFFECT_ID"
		}
		if dispatchName == "command" {
			command.ArgsUsage = "\"TEXT\""
		}
		if dispatchName == "themes apply" {
			command.ArgsUsage = "THEME.json"
		}
		if dispatchName == "part power" {
			command.ArgsUsage = "on|off"
		}
		return command
	}
	devices := leaf("devices", "Device inventory and diagnostics (defaults to list)", "devices",
		&ucli.BoolFlag{Name: "watch", Usage: "Keep refreshing the list (list only)"},
		&ucli.DurationFlag{Name: "interval", Value: time.Second, Usage: "List refresh interval with --watch; no extra LAN requests"},
		&ucli.StringSliceFlag{Name: "filter", Usage: "List filter key=value; repeat for AND across keys, OR within a key"})
	devices.Commands = []*ucli.Command{
		leaf("parts", "Inspect logical light parts and freshly observed brightness", "parts", target(), &ucli.BoolFlag{Name: "fresh", Value: true, Usage: "Refresh state; use --fresh=false for cached observations"}),
		leaf("color", "Set partial HSBK values for one device or light part; preserve power", "part color", target(),
			&ucli.StringFlag{Name: "part", Value: "all", Usage: "Light part: all, main, uplight"},
			&ucli.Float64Flag{Name: "hue", Usage: "Hue in degrees (0..360)"},
			&ucli.Float64Flag{Name: "saturation", Usage: "Saturation percent (0..100)"},
			&ucli.Float64Flag{Name: "brightness", Usage: "Brightness percent (0..100); zero does not change power"},
			&ucli.Float64Flag{Name: "kelvin", Usage: "Color temperature in Kelvin"},
			&ucli.DurationFlag{Name: "duration", Usage: "Color transition duration"}),
		leaf("power", "Express on/off intent for one device or light part", "part power", target(),
			&ucli.StringFlag{Name: "part", Value: "all", Usage: "Light part: all, main, uplight"},
			&ucli.DurationFlag{Name: "duration", Usage: "Power/color transition duration"},
			&ucli.Float64Flag{Name: "fallback-brightness", Usage: "Explicit activation fallback percent (>0..100), only if no brightness is available"}),
		leaf("list", "List discovered devices; optionally refresh the inventory", "devices"),
		leaf("inspect", "Inspect cached device state and estimated uptime", "inspect", target()),
		leaf("buffers", "Read matrix frame buffers without changing colors or power", "buffers", target(), &ucli.IntSliceFlag{Name: "buffer", Value: []int{0, 1, 2}, Usage: "Buffer index 0..2; may repeat; defaults to 0, 1, 2"}),
		leaf("ping", "Collect sequential echo samples and latency statistics", "ping", target(), &ucli.IntFlag{Name: "count", Value: 5, Usage: "Number of samples"}),
		leaf("stream", "Stream device events, optionally for selected devices", "stream", &ucli.StringFlag{Name: "target", Usage: "Optional comma-separated selectors; omit or use all for every device"}),
	}
	app := &ucli.Command{
		Name: "lifxlan", Usage: "Diagnose and control LIFX devices directly over LAN",
		Writer: out, ErrWriter: diagnostic,
		// main owns printing errors and selecting the process exit status.
		ExitErrHandler: func(context.Context, *ucli.Command, error) {},
		Flags: []ucli.Flag{
			&ucli.StringFlag{Name: "output", Value: "text", Usage: "Output format: text or json"},
			&ucli.DurationFlag{Name: "discover-for", Value: 3 * time.Second, Usage: "Discovery window (not a state-readiness guarantee)"},
			&ucli.DurationFlag{Name: "timeout", Value: 3 * time.Second, Usage: "Timeout for each ping, snapshot or matrix buffer read"},
			&ucli.BoolFlag{Name: "verbose", Usage: "Controller diagnostic logging to stderr"},
		},
		Commands: []*ucli.Command{
			devices,
			{Name: "themes", Usage: "Apply static palettes with deterministic layouts", Commands: []*ucli.Command{
				leaf("apply", "Preview or apply a theme; power is preserved", "themes apply",
					&ucli.StringSliceFlag{Name: "target", Usage: "Comma-separated serial/label/group/location/all selectors; may repeat; only lights are selected"},
					&ucli.StringSliceFlag{Name: "filter", Usage: "Select lights with list-style key=value filters; do not combine with --target"},
					&ucli.BoolFlag{Name: "dry-run", Usage: "Print resolved color packet plan without control sends"},
					&ucli.DurationFlag{Name: "duration", Value: time.Second, Usage: "Color transition duration; power is unchanged"}),
			}},
			leaf("command", "Compile and send one quoted natural-language command", "command", &ucli.BoolFlag{Name: "dry-run", Usage: "Preview targets and packets without control sends (discovery still uses LAN)"}),
			leaf("snapshot", "Capture complete observed state as JSON", "snapshot", target(), &ucli.BoolFlag{Name: "fresh", Usage: "Require observations after capture starts"}),
			{Name: "effects", Usage: "List or run registered lighting effects", Commands: []*ucli.Command{
				leaf("list", "List presets and configuration parameters (offline)", "effects list"),
				leaf("run", "Run one effect; preserve power and restore state on exit", "effects run", target(),
					&ucli.StringFlag{Name: "config", Usage: "Effect config JSON file"},
					&ucli.DurationFlag{Name: "step", Value: 100 * time.Millisecond, Usage: "Frame interval (minimum 20ms)"},
					&ucli.DurationFlag{Name: "duration", Usage: "Run duration; zero runs until interrupted"},
					&ucli.BoolFlag{Name: "restore", Value: true, Usage: "Restore freshly captured state on exit"}),
			}},
		},
	}
	return app.Run(ctx, append([]string{"lifxlan"}, args...))
}
