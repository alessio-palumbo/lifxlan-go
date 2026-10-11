# Main and uplight colour targeting

`device.LightParts(d)` returns descriptors for main and, when the registry says
it exists, uplight. Main keeps the existing visible-zone count; LIFX Ceiling 13" (PID 265)
has 56 main zones and one uplight zone. Uplight is single-zone. Unknown main
geometry is represented by `ZonesKnown=false`, not a fabricated zero. These are
capabilities/cached geometry, not proof that colours were observed. Descriptors
are independent values and do not duplicate serials, addresses or sessions.

```go
parts := device.LightParts(d)
brightness := 40.0
err := ctrl.SetLightPartColor(ctx, d.Serial, device.LightPartUplight,
    controller.LightStateUpdate{Brightness: &brightness, Duration: time.Second},
    controller.SnapshotOptions{Timeout: 3 * time.Second, RequireFresh: true})
```

The selectors are `LightPartMain`, `LightPartUplight`, and `LightPartAll`. An empty
selector means all. Reusing LightStateUpdate gives the same partial HSBK semantics
as existing SetState: nil fields retain values; an explicit zero brightness sets
black. Power must be nil here; use SetState for explicit device-wide power.
Unsupported uplights, invalid selectors/values, or incompatible geometry fail.
Kelvin validation follows existing SetState temperature bounds; white-only lights
receive zero saturation. No implicit device on/off or firmware effect stopping
occurs. This colour API is distinct from the explicit on/off policy below.

All uses the existing optional-waveform message builder. Main on a device with
no uplight is equivalent to all. Targeted main/uplight updates on current uplight
ceilings first capture complete observed physical state; SnapshotOptions control
timeout, polling and freshness. Complete cached state is accepted by default;
RequireFresh is recommended when other controllers may be active. Missing state
or observation invalidation causes no colour writes. Endpoint/session transitions
abort pending part updates rather than writing plans to the wrong session.

The internal planner uses existing surface/orientation mapping to select emitters
and existing matrix message builders to encode them. It copies original physical
HSBK values, changing only requested fields on selected cells. Main leaves the
uplight untouched; uplight leaves main and padding untouched. Unselected values
retain exact protocol precision, including black. Raw buffers and main surface
geometry are unchanged. Capsule fixtures cover existing logical/send reshaping;
real capsule hardware verification remains advisable. Current uplight mapping is
limited to registry-described, single-chain ceilings rather than guessing at
future chained/multi-light layouts.

Matrix part updates send complete physical frames for preservation. Larger-than-
64 frames still use buffer 1 for ordinary staging; there is no retained backup
buffer. Preservation is based on captured state,
not an atomic device transaction, so another writer can overwrite it. Batches
are serialized with other sends in this session, but external clients are not
coordinated. Send failures return StateSendError with Sent/Total; sends are not
ACKs or confirmation of application. Cancellation stops between packets, not an
already-issued datagram. No automatic rollback or retry is attempted.

Existing SetState, messages.SetColor, type-specific physical packet builders,
themes and effect renderers retain their old semantics. In particular, raw matrix
rendering still blanks hidden slots and is not automatically uplight-preserving.
Part-aware spatial rendering remains separate work.
Callers use logical selection; physical addressing remains an internal backend
detail so future protocol-defined parts can be supported additively.

## Client integration boundary

Use SetLightPartColor for uniform/partial colour or absolute brightness changes
on a selected part. Brightness zero makes that part dark and preserves the other
part, but does not change device power or remember brightness for later. Global
power uses SetState. Part on/off intent uses SetLightPartPower; clients must not
infer its policy from ordinary successful colour writes.

## Explicit part on/off

```go
err := ctrl.SetLightPartPower(ctx, serial, device.LightPartUplight, false,
    controller.LightPartPowerOptions{Duration: 250 * time.Millisecond})

state, err := ctrl.GetLightPartState(serial, device.LightPartUplight)
// state.Known guards observation readiness. state.On is effective part state;
// state.DevicePoweredOn distinguishes shared power from stored brightness.
```

Hikari selects a part and on/off intent; it does not calculate brightness means,
map cells, or sequence power packets. The current registry-defined uplight backend
emulates this policy, without claiming native independently powered subdevices:

- Off while another part is lit: zero only the selected visible emitters.
- Last lit part off: switch device power off without zeroing its stored pattern.
  If main was the last lit part, confirm fresh device-off state before setting
  dormant uplight brightness to the arithmetic mean of visible main cells.
  Hidden/padding/uplight cells are excluded; valid black main cells are included.
- On while another part is lit: a zeroed part inherits its brightness. Existing
  nonzero selected patterns are retained rather than flattened.
- On from device-off: use retained selected brightness, or inherit the other
  part's retained brightness when selected state is black. Prepare all colours
  (zero the unselected part) before powering on, so part-only activation does
  not illuminate retained colours on the other part.
- All-off preserves both stored patterns; all-on retains nonblack parts and
  fills completely black parts by inheritance. Off while already device-off
  is a no-op, preserving the stored brightness source.

There is no historical per-part brightness cache. Re-enabling a zeroed matrix
sets uniform inherited brightness, retaining hue/saturation/Kelvin, not an old
brightness distribution. Colour writes to black do not trigger power policy.
No active/retained positive brightness returns ErrNoInheritedBrightness before
control writes. An explicit FallbackBrightness (for example 1%) can opt into a
fallback; nil never silently invents it. Values must be finite, >0 and <=100.
Positive inherited values below wire precision are limited to one nonzero
protocol unit, not a fabricated 1% level.

Fresh complete observations are required for emulation. Timeout bounds the whole
operation, including waiting for another part operation; zero defaults to 3s plus
Duration. Waiting for confirmed off prevents intentionally preparing a dormant
uplight while the device still reports on. A timeout after the off send can leave
the device off without completing dormant preparation; StateSendError reports
partial control progress. There are no retries of control writes or rollback.
Endpoint/session changes abort subsequent phases. Part operations share a
context-aware gate, and each batch uses the session send lock; raw SetState,
renderers, firmware effects and external clients remain potential competing
writers. This is not an atomic hardware transaction.

GetLightPartState reads cache only and returns Known=false for incomplete receipt
coverage. On uplight devices, main/uplight brightness is the corresponding mean,
all reports the brighter part, and On requires shared power plus positive selected
brightness. Emulated is true. Ordinary devices without uplight use existing native
device-power semantics; On reflects native power, reported global brightness is
provided, and Emulated is false. The helpers do not advertise extra devices or
native subdevice IDs; future backends can change mapping behind logical selectors.

No hidden buffers are reserved for backup. Larger frames still use existing
buffer-1 staging. The on/off policy has been manually verified on a LIFX Ceiling
13" (PID 265); the automated tests use simulated state responses, not live writes.

Raw SetMatrixColorsFromSlice and current frame renderers remain physical/full-
surface operations. They are not automatically main-only or uplight-preserving.
Do not alternate raw main frames and uplight colour commands expecting them to
remain independent: a later raw frame can overwrite uplight. Part-aware frame
composition is required for main themes/images/effects with independent uplight;
that mapping/preservation belongs in the library, not duplicated in clients.
