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
1500–9000. Zero brightness is valid; omitting brightness means zero, not
“preserve”. Invalid files fail before discovery.

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
Stopping those effects, or cancelling another application-side animation, is
the caller's responsibility; theme planning does neither.
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

### Optional per-cell brightness preservation

`Theme.Plan` keeps its palette-brightness behaviour. `Theme.PlanWithOptions`
adds an explicit runtime policy without changing the target-free theme file:

```go
// selected must contain complete observed device state. For controller-managed
// devices, first successfully call CaptureStateSnapshot for every selected
// serial, then read the updated devices. Use RequireFresh when preservation
// must reflect observations received for this application rather than cache.
initial := make(map[device.Serial]effects.Frame, len(selected))
for _, d := range selected {
    frame, err := effects.FrameFromDeviceState(d, 0)
    if err != nil {
        return err
    }
    initial[d.Serial] = frame
}
plan, err := theme.PlanWithOptions(selected, transition, themes.PlanOptions{
    Brightness: themes.PreserveBrightness,
    InitialFrames: initial,
})
if err != nil {
    return err
}
// Preview and live rendering must consume this same plan; do not recapture or
// regenerate a different plan between preview and send.
```

Empty options and `PaletteBrightness` use the palette's numeric brightness,
including zero. `PreserveBrightness` changes hue, saturation and Kelvin as
usual, but retains each initial cell's brightness, not the device-wide light
state or an average. Single-zone lights retain their single brightness value.
Black cells remain black, and white-only adaptation and Kelvin clamping still
apply. Initial frames are not persisted in Theme definitions.

Preservation requires exactly one initial frame for every selected serial and
no extra serials. Frames must have exactly matching logical dimensions and
colour counts; no resampling or default substitution is performed. Only their
brightness is consumed and validated (finite, 0–100); hue, saturation, Kelvin
and duration are ignored. Both inputs and output colour slices remain
independent. Unknown policies and initial frames supplied in palette mode
(including a non-nil empty map) are errors.

Generate initial frames with the same device metadata used for planning so
irregular rows, orientation, and chain placement match. Existing helpers handle
logical padding and hidden cells; the renderer retains its existing blanking of
non-emitting cells. Completeness is a caller/controller obligation, not something
the pure planner can authenticate: `FrameFromDeviceState` can pad missing data,
and allocated buffers are not proof of observation. Do not use it as a readiness
check or infer missing state from brightness values.

Both planning methods bound geometry before surface allocation. `MaxFrameCells`
is 65,536 per target, applied to raw physical geometry/buffers and the resulting
logical frame. `MaxPlanCells` is 1,048,576 logical cells across the plan. Excessive
geometry fails rather than allocating impractical frames. These are planner
safety limits, not claims about protocol/device capacity. All targets are
validated before output frames are allocated; planning never sends controls.
`MaxPlanTargets` limits one plan to 4,096 devices before metadata is copied.

This option is currently a library API only. `themes apply` continues to use
palette brightness. The planner still does not change power, stop effects,
capture/restore scenes, or make a multi-device application atomic. Callers
should compile and validate every target's packets before control writes;
successful sends do not acknowledge application and partial failures can leave
a partially applied theme.

### Deterministic application variations

Runtime `PlanOptions` can vary an application without editing or reordering the
saved theme palette:

```go
opts := themes.PlanOptions{
    Variation: 2,
    Reverse: false,
    Seed: 42,
    MatrixLayout: themes.MatrixSpatial,
}
plan, err := theme.PlanWithOptions(selected, transition, opts)
```

- `Variation` is a uint64 palette offset. It wraps across the existing stop order
  (base, accents, backgrounds). Successive values select successive colours on
  a single-zone light; groups distribute the offset palette in stable serial
  order among single-zone targets. Other light types do not consume their slots.
- `Reverse` reverses the offset palette. On gradients this reverses traversal,
  including two-colour gradients; it also affects stepped/solid assignments.
  Palette offset changes the stop ordering, not individual pixel randomness;
  gradients still interpolate smoothly using the shortest hue path.
- `MatrixLayout` is empty/`MatrixThemeLayout` by default, retaining the saved
  layout and axis. `MatrixSpatial` overrides them for matrices only. It samples
  the palette through a fixed 3×3 scalar field with smooth interpolation,
  producing coherent two-dimensional colour regions. This is not independent
  random colour selection per pixel. Strips and single-zone lights retain their
  usual layouts.
- `Seed` controls only the spatial field. Zero is a valid deterministic seed,
  not a request for randomness. Seed, full variation value, and device serial
  determine the field, independently of target input order and unrelated
  targets. Palette ordering wraps; spatial patterns do not necessarily repeat
  after one palette cycle.

The spatial field spans the existing logical device surface across matrix
chains; it does not restart per tile or model physical room placement. Existing
surface metadata, irregular row offsets, hidden-cell blanking, orientation and
packet adapters retain their responsibilities. A one-colour palette remains
uniform; degenerate dimensions remain valid.

Zero variation options retain existing output. Brightness preservation composes
with every variation: the planner replaces brightness from the corresponding
initial cells after generating the varied colours, retaining black cells and
relative brightness. White-only adaptation and Kelvin clamping still apply.

No counter, hidden randomness, network calls or mutable package state is introduced.
The caller supplies variation/seed values and uses the same options, targets,
metadata and initial frames for preview and application. Changing target
membership can still change serial-based colour assignments. These options are
currently library-only; CLI defaults and theme file definitions are unchanged.
