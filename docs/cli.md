# Direct LAN CLI

`lifxlan` uses the library directly, without the HTTP daemon or another client.
Its nested commands and help use `urfave/cli` v3; output formatting uses the Go
standard library. Build it with `go build -o lifxlan ./cmd/lifxlan`, or use
`go run ./cmd/lifxlan` in place of `lifxlan` below.

```sh
lifxlan help
lifxlan effects --help
lifxlan effects run --help
lifxlan devices --help
lifxlan devices list --discover-for 5s
lifxlan devices list --output json
lifxlan devices list --watch --interval 1s
lifxlan devices list --watch --output json
lifxlan devices list --filter location=Office --filter type=matrix
lifxlan devices inspect --target Desk
lifxlan devices ping --target Desk --count 10 --timeout 1s
lifxlan devices stream
lifxlan devices stream --target Desk
lifxlan command --dry-run "Desk blue 50%"
lifxlan command "Desk blue 50%"
lifxlan snapshot --target Desk --fresh
lifxlan effects list
lifxlan effects run --target Desk --duration 10s breathe
lifxlan effects run --target Desk color_cycle
```

Shared flags (`--output`, `--discover-for`, `--timeout`, `--verbose`) can go before
the subcommand or alongside its flags. Keep flags before positional arguments.
Target selection is an exact,
case-insensitive label or a 12-digit hexadecimal serial. Duplicate labels are
rejected; use a serial to disambiguate. Inspect, ping, snapshot, and effect
commands require an explicit target. Natural-language commands use the existing
parser's selectors, including groups and locations; always preview unfamiliar
commands with `--dry-run` because the parser is deliberately forgiving.

`devices` groups `list`, `inspect`, `ping`, and `stream`. Bare `devices` remains
shorthand for `devices list`, including list flags. The former top-level
`inspect`, `ping`, and `watch` commands are replaced by the grouped commands;
event streaming is named `stream` to distinguish it from the list's `--watch`.

## Inventory filtering and targeted streams

`devices list --filter key=value` accepts `serial`, `label`, `location`, `group`, `product_id`,
`firmware_version`, and `type`. Repeat filters: different keys combine with AND,
while values of the same key combine with OR. Text matching is exact and case-
insensitive (not substring matching); commas are literal, not separators.
Product IDs are positive uint32 decimal numbers, firmware is `major.minor`, and
type is `single_zone`, `multi_zone`, `matrix`, or `switch` as shown in the table.
Hybrid lights match their light type, not a separate `hybrid` value. Invalid
keys/values fail before opening the controller. Unknown metadata never matches
a requested value. Filters affect text and JSON equally and are reevaluated on
every watched refresh, so devices appear when matching metadata arrives. They
do not alter discovery, polling, or device state.

Serial filters require 12 hexadecimal digits and ignore case. Label filters are
exact and case-insensitive, but unlike `--target` they may match duplicate labels.
Repeated serial or label values select any matching value, still ANDed with other
keys. A watched label filter is reevaluated when a label arrives or changes.

```sh
lifxlan devices list --filter location=Office --filter group=Desk --filter group=Ceiling
lifxlan devices list --watch --filter type=matrix --filter firmware_version=4.110
lifxlan devices list --filter label=BR30
lifxlan devices list --watch --filter serial=d073d554ecf6
```

`devices stream` subscribes immediately and emits all device events by default.
With `--target SERIAL`, filtering starts immediately, including for a device
that is not yet discovered. With `--target LABEL`, the command waits the
`--discover-for` window, resolves the label uniquely, and subscribes with the
selection bound to that serial (later renames do not change it). Missing or
ambiguous labels fail. Subscription `snapshot_complete` and `resync_required`
control events remain visible even in targeted streams; they do not identify a
device and indicate subscription state, not color-state completeness. Selection
is client-side: the controller's existing subscription model is unchanged.

Human stream lines include the CLI display timestamp, revision, event kind,
serial, and label. Initial cached entries are labelled `initial`, not live
`added` events. Updates name their changed categories and show relevant current
values: light power and HSBK color, metadata, Wi-Fi signal, firmware effects,
relays, buttons, and uptime. Matrix/multizone updates show cached zone counts,
brightness ranges, and at most three preview colors rather than every zone.
These samples can include unpopulated buffers and do not establish completeness.
Categories are not per-field diffs: a `light` update prints both power and color
even if only one changed. JSON retains the existing full `DeviceEvent` format.

Use `devices list --watch` as a dashboard of the current inventory: it redraws
even when state is unchanged and updates estimated uptime. Use `devices stream`
as a change log: it preserves observed transitions between display refreshes and
is quiet when values are unchanged. Neither is a packet trace, heartbeat, or
guarantee that all transient physical changes were observed; the controller only
emits state changes it receives through its normal polling and replies.

Output defaults to human-readable text: an inventory table, labelled inspection
fields, ping samples followed by loss and min/average/max latency, readable
command plans, and concise event lines. Inspection summarises matrix/zone buffers
rather than dumping every pixel. Device-provided control characters are stripped
from text output to prevent labels from controlling the terminal.

The inventory columns are serial, IP, label, location, group, type, zones,
product ID, firmware, power, Wi-Fi RSSI/SNR, and estimated uptime. The separate
`ZONES` column includes every addressable matrix pixel across the chain: five
64-zone tiles show `320`. Single-zone lights show `1`, switches `-`, and unknown
counts `?`. JSON inventory and inspection include a numeric `ZoneCount`, or
`null` for unknown/not-applicable counts (never a fabricated zero). Counts describe
cached geometry/buffer sizes, not received color coverage. Uptime is estimated
from the controller's boot-time baseline; missing uptime or IP shows `-`.
Firmware and signal come from the controller
cache; `-` indicates an unset value. The library interprets Wi-Fi signal as RSSI
or SNR depending on firmware, so the column deliberately shows the raw value
without claiming a single unit. A zero signal is also the controller's unset
sentinel, so the CLI cannot distinguish it from an observed zero.

One-shot inventory output is time-based: the CLI opens a controller, waits the full
`--discover-for` window (default 3s), then reads its cache once. It does not
return early when a device responds or extend the window for missing metadata.
Untargeted/serial-targeted `devices stream` starts immediately; help and effect listing are offline.

One-shot `devices list` shows a spinner on stderr during discovery when both output
streams are terminals and text mode is selected. It is cleared before the result
or on cancellation. JSON, redirected output, `TERM=dumb`, and `--verbose` disable
animation to keep data and diagnostic logs clean.

`devices list --watch` displays immediately, skipping the initial discovery wait
(`--discover-for` does not delay watch mode). An empty initial cache displays a
waiting message while discovery continues. It keeps the controller alive
and refreshes the whole inventory every `--interval` (default 1s). The interval
only reads the local cache; it does not send additional discovery/state requests
or change the controller's normal polling periods. Late responses and removed or
new devices appear on subsequent refreshes. Text redraws in place on a terminal;
files/pipes receive appended timestamped tables. With `--output json`, each line
is an object containing `Timestamp` and the full `Devices` array. Ctrl+C stops
the loop and closes the controller. `--interval` requires `--watch` and a positive
duration. Hybrid lights (lights with buttons) also receive numeric zone counts
or `?` while geometry is unknown; only non-light devices show `-`.

Use `--output json` for structured output, retaining full cached device fields
and effect definitions. Ping and stream emit one JSON object per line; other data
commands emit a single indented JSON value. Snapshot capture always emits JSON,
even with `--output text`, because it is a data artifact. Effects run emits no
status banner in JSON mode. Errors and optional `--verbose` controller logging
go to stderr, leaving JSON stdout clean. Inspection contains cached
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
`devices stream` reports existing device events, including resync-required notifications;
it is not a stream of every incoming packet or unchanged state response.
