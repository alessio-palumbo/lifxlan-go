# Static themes

Themes apply a named HSBK palette once, leaving colors in place after the command
exits. They are not running effects and do not turn lights on or off. Image
palette extraction, built-in theme catalogs, and saving/exporting device themes
are separate follow-ups.

Start with the supplied example and preview before sending:

```sh
go run ./cmd/lifxlan themes apply --target BR30 --dry-run examples/themes/evening.json
go run ./cmd/lifxlan themes apply --target BR30 --duration 2s examples/themes/evening.json
go run ./cmd/lifxlan themes apply --target Desk --target Beam --dry-run examples/themes/evening.json
go run ./cmd/lifxlan themes apply --target "BR30,group:Office" --dry-run examples/themes/evening.json
go run ./cmd/lifxlan themes apply --target all --dry-run examples/themes/evening.json
go run ./cmd/lifxlan themes apply --filter group=Office --dry-run examples/themes/evening.json
```

Selection must use either `--target` selectors or repeatable list-style `--filter`
values, never both. There is no implicit all-device selection: use `--target all`
explicitly. Targets accept comma-separated serials, labels, groups, locations,
and `all`; the flag can also be repeated. Bare names match exactly and
case-insensitively across those fields. Typed selectors such as `label:Desk` or
`group:Office` disambiguate names. Duplicate labels may select multiple lights;
overlapping selections are deduplicated.

Targets and filters select only light-capable devices, skipping switches.
Every target token must match an eligible device, otherwise the command fails
before color writes. For names containing commas, use a literal `--filter`
value instead, since commas delimit target selectors.

## Theme file and layout

```json
{
  "name": "Evening",
  "palette": {
    "base": [
      {"hue": 30, "saturation": 80, "brightness": 40, "kelvin": 3500},
      {"hue": 220, "saturation": 80, "brightness": 30, "kelvin": 3500}
    ]
  },
  "layout": "gradient",
  "axis": "horizontal"
}
```

The palette reuses `effects.Palette`: stops are `base` followed by `accents`,
then `backgrounds`. At least one color is required. Hue is 0–360 degrees,
saturation and brightness are 0–100 percent, and kelvin must be explicitly
1500–9000. Zero brightness is valid. Invalid files fail before discovery.

- `gradient` (default): smoothly interpolate stops across the surface, taking
  the shortest hue path and interpolating saturation, brightness, and kelvin.
- `steps`: divide the axis into contiguous palette bands, without interpolation.
- `solid`: fill each target with one palette color, cycling in serial order.

`axis` is `horizontal` by default or `vertical`. Matrix layouts use the existing
logical surface, with tile chains laid out side by side and device orientation
mapping handled by the renderer. On a strip, horizontal spans its zones; vertical
has a single row and therefore uses the first stop. Single-zone lights always
receive successive palette stops in serial order, independently of input order
or the order returned by discovery. Changing the selected device set can change
assignments. Non-color lights receive zero saturation; kelvin is clamped to each
device's known temperature range. Preview shows the resulting values.

## Safety and timing

After the discovery window, the command waits for complete observed cached state
before using device geometry. `--timeout` bounds that capture. This is not a
fresh/atomic scene read, and includes powered-off matrices. Missing state aborts
without color writes. All selected targets and their packet plans are validated
before the first control send.

`--dry-run` still uses discovery and state queries but sends no color/power
commands. Human output is a compact target/color preview. `--output json` returns
the full target-bound packet plan, with no status banner. Output failure prevents
control sends. `--duration` is the color transition duration (default 1s), not
a time limit or scheduled restore. The command does not wait for the transition
to finish. Transitions must fit the protocol's uint32 millisecond range.

Power is preserved: powered-off lights receive stored colors but remain off.
Existing firmware effects are not stopped and can affect what is visible.
The application is sequential, not synchronized/atomic across devices. Sends
are not acknowledgements. Network/send failures or cancellation can leave a
partially applied theme; no automatic rollback is attempted and no power is
changed to hide the partial result. Other controllers can overwrite colors.

## Library use

`pkg/themes.Theme.Validate` validates a description, and `Theme.Plan` turns
caller-provided devices into independent `themes.Application` frames without
network access. The planner validates basic geometry but cannot know whether
colors or metadata were observed. Controller users should capture complete state
before reading the devices used to plan. Render each frame with
`effects/adapters.NewRendererForDevice`, or collect the generated messages for a
preview before sending. Planning does not mutate device or palette inputs.
