# Direct LAN CLI

`lifxlan` uses the library directly, without the HTTP daemon or another client.
It introduces no new dependencies. Build it with `go build -o lifxlan
./cmd/lifxlan`, or use `go run ./cmd/lifxlan` in place of `lifxlan` below.

```sh
lifxlan help
lifxlan devices --discover-for 5s
lifxlan inspect --target Desk
lifxlan ping --target Desk --count 10 --timeout 1s
lifxlan watch
lifxlan command --dry-run "Desk blue 50%"
lifxlan command "Desk blue 50%"
lifxlan snapshot --target Desk --fresh
lifxlan effects list
lifxlan effects run --target Desk --duration 10s breathe
lifxlan effects run --target Desk color_cycle
```

Flags go before positional arguments. Target selection is an exact,
case-insensitive label or a 12-digit hexadecimal serial. Duplicate labels are
rejected; use a serial to disambiguate. Inspect, ping, snapshot, and effect
commands require an explicit target. Natural-language commands use the existing
parser's selectors, including groups and locations; always preview unfamiliar
commands with `--dry-run` because the parser is deliberately forgiving.

Output is JSON, with one JSON object per line for ping and watch. Errors and
optional `--verbose` controller logging go to stderr. Inspection contains cached
device values and estimated uptime, which can be unknown. The discovery window
(default 3s) is not a readiness guarantee or proof that every LAN device was
found. Increase it for a busy network. Snapshot and effect execution separately
wait for complete observed state, bounded by `--timeout` (default 3s).

Dry-run still opens the controller and performs normal discovery/state queries,
but sends none of the compiled control messages. It prints resolved serials and
payload values. A successful non-dry-run command means sending succeeded, not
that the device acknowledged or applied the state. Ping samples run sequentially
and report individual failures; any failed sample makes the command exit with an
error after collecting the remaining samples.

## Effects and restoration

Custom single-zone effects already fit the library's `Effect` interface and
`SingleZoneRenderer`. Two registered temporal presets now complement that:

- `color_cycle`: hold each base/accent palette color and blend to the next using
  the shortest hue path. Defaults: 1s hold, 2s transition per color.
- `breathe`: smoothly pulse absolute brightness between bounds, preserving hue,
  saturation, and kelvin. Defaults: 10–100% over a 4s cycle.

Both fill the whole logical surface uniformly, so they also support strips and
matrices. Both are deterministic and implement `PhaseEffect` for offline
sampling. Registry configs reject invalid timing and brightness bounds.

An effect command captures fresh, complete state before starting and uses the
updated capabilities to construct its renderer. Capture failure aborts the
command without changing colors. The effect does not turn a light on; a powered-
off target can receive colors without showing them. State is restored on normal
completion, Ctrl+C, SIGTERM, or rendering failure, using a separate timeout even
when the run context has been cancelled. Restoration errors are reported and
cause a nonzero exit. `--restore=false` explicitly leaves the final effect state.
SIGKILL, process crashes, and network loss can prevent restoration. Restore sends
are not application-level acknowledgements. Snapshots restore colors and power,
not firmware effect configuration or changes made concurrently by another app.

The default step is 100ms. Steps below 20ms are rejected as a basic traffic guard,
not a guarantee of a safe frame rate for every device. Matrix frames require
multiple datagrams; prefer a slower step when troubleshooting packet loss.

For configuration, supply an `effects.Config` JSON file whose ID matches the
effect argument:

```json
{
  "id": "breathe",
  "params": {
    "color": {"hue": 30, "saturation": 80, "brightness": 100, "kelvin": 3500},
    "period": "6s",
    "min_brightness": 5,
    "max_brightness": 60
  }
}
```

```sh
lifxlan effects run --target Desk --config breathe.json --duration 20s breathe
```

Theme application, image palette extraction, an interactive shell/TUI, packet
tracing, and snapshot-file restore are not included in this first CLI milestone.
`watch` reports existing device events, including resync-required notifications;
it is not a stream of every incoming packet or unchanged state response.
