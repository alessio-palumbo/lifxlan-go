# Pattern-preserving Breathe

`effects.NewPatternBreathe` applies one smooth brightness envelope to an immutable
logical frame. Each cell keeps its hue, saturation, and Kelvin. Its brightness
is the original brightness multiplied by the current envelope, clamped to
0–100%. Relative brightness is preserved until clamping occurs. Black cells
stay black; a completely black initial pattern remains invisible.

This is a direct library constructor, not another registered preset. Existing
`NewBreathe`, the `breathe` registry entry, and CLI behaviour remain unchanged:
they use one uniform colour and absolute brightness percentages. Color Cycle
also retains its uniform behaviour on all light types.

```go
// d must contain complete observed state, not merely allocated zone buffers.
initial, err := effects.FrameFromDeviceState(d, 0)
if err != nil {
    return err
}
pulse, err := effects.NewPatternBreathe(effects.PatternBreatheConfig{
    InitialFrame: initial,
    Period: 4 * time.Second,
    MinMultiplier: 0.1,
    MaxMultiplier: 1,
})
if err != nil {
    return err
}
// Use pulse with effects.NewRunner and a target-specific renderer, or sample
// pulse.FrameAtPhase for an offline preview. No network access happens here.
```

The initial frame defines the surface: 1×1 for a single-zone light, a row for a
strip, or the logical matrix layout returned by `FrameFromDeviceState`. Positive
dimensions must exactly match the colour count. HSB values must be finite and
within their documented ranges (hue 0–360, saturation/brightness 0–100).
Kelvin must be 1500–9000; all-zero colours are also accepted for blank matrix
padding. The constructor copies the colours; changing the input or an output
frame cannot change future samples.

Multipliers are factors, not percentages: 1 preserves original brightness, 0
is black, and values above 1 brighten up to the 100% clamp. They must be finite,
nonnegative, and ordered minimum ≤ maximum. Equal bounds yield a constant
pattern; two zero bounds intentionally yield black. Period defaults to 4s when
zero; negative values fail. Samples start at minimum, reach maximum halfway
through the cycle, and return to minimum. `Reset` restarts timing using the same
original pattern; it never recaptures state or cumulatively dims colours.

Capture, power, cancellation, and restoration belong to the caller. For a live
device, capture complete state before constructing the frame; request fresh
state if that is required for the application's semantics, then read the
updated device. Keep the captured restore snapshot independently. Powered-off
devices can supply patterns and receive frames without being turned on.

Persist only settings such as period and multiplier bounds, not the runtime
initial frame. Capture or supply that frame when starting or previewing. A
client can offer one Breathe UI with “Selected colour” and “Current pattern”
sources without changing the library registry. Matrix packet counts still
depend on the full frame size: this does not introduce a cheaper device-side
brightness animation.
