package cli

import (
	"context"
	"io"
	"time"

	ucli "github.com/urfave/cli/v3"
)

func run(ctx context.Context, args []string, out, diagnostic io.Writer, create factory) error {
	target := func() ucli.Flag {
		return &ucli.StringFlag{Name: "target", Usage: "Exact device label or 12-digit serial", Required: true}
	}
	leaf := func(name, usage, dispatchName string, flags ...ucli.Flag) *ucli.Command {
		command := &ucli.Command{Name: name, Usage: usage, Flags: flags, DisableSliceFlagSeparator: true, Action: func(ctx context.Context, cmd *ucli.Command) error {
			return dispatch(ctx, dispatchName, cmd, out, diagnostic, create)
		}}
		if dispatchName == "effects run" {
			command.ArgsUsage = "EFFECT_ID"
		}
		if dispatchName == "command" {
			command.ArgsUsage = "\"TEXT\""
		}
		return command
	}
	devices := leaf("devices", "Device inventory and diagnostics (defaults to list)", "devices",
		&ucli.BoolFlag{Name: "watch", Usage: "Keep refreshing the list (list only)"},
		&ucli.DurationFlag{Name: "interval", Value: time.Second, Usage: "List refresh interval with --watch; no extra LAN requests"},
		&ucli.StringSliceFlag{Name: "filter", Usage: "List filter key=value; repeat for AND across keys, OR within a key"})
	devices.Commands = []*ucli.Command{
		leaf("list", "List discovered devices; optionally refresh the inventory", "devices"),
		leaf("inspect", "Inspect cached device state and estimated uptime", "inspect", target()),
		leaf("ping", "Collect sequential echo samples and latency statistics", "ping", target(), &ucli.IntFlag{Name: "count", Value: 5, Usage: "Number of samples"}),
		leaf("stream", "Stream device events, optionally for one device", "stream", &ucli.StringFlag{Name: "target", Usage: "Optional exact label or 12-digit serial; omit for all devices"}),
	}
	app := &ucli.Command{
		Name: "lifxlan", Usage: "Diagnose and control LIFX devices directly over LAN",
		Writer: out, ErrWriter: diagnostic,
		// main owns printing errors and selecting the process exit status.
		ExitErrHandler: func(context.Context, *ucli.Command, error) {},
		Flags: []ucli.Flag{
			&ucli.StringFlag{Name: "output", Value: "text", Usage: "Output format: text or json"},
			&ucli.DurationFlag{Name: "discover-for", Value: 3 * time.Second, Usage: "Discovery window (not a state-readiness guarantee)"},
			&ucli.DurationFlag{Name: "timeout", Value: 3 * time.Second, Usage: "Timeout for each ping or snapshot"},
			&ucli.BoolFlag{Name: "verbose", Usage: "Controller diagnostic logging to stderr"},
		},
		Commands: []*ucli.Command{
			devices,
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
