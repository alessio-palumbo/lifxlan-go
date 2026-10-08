# Caller-timed matrix frame streaming

`messages.NewMatrixFrameSequence` accepts physical HSBK frames and returns a
looping function yielding the next frame's complete packet batch. Construction
validates and copies all frames; it sends nothing and allocates no device buffers.
Each call produces independent messages, safe to mutate/send without changing
the stored frames or later output. Calls must be serialized.

```go
next, err := messages.NewMatrixFrameSequence(0, 1, width, frames, transition)
if err != nil {
    return err
}
// At each caller-chosen interval:
batch := next()
for _, msg := range batch {
    if err := ctx.Err(); err != nil {
        return err
    }
    if err := ctrl.Send(serial, msg); err != nil {
        return err
    }
}
```

The first call returns frame 0; after the last frame the sequence loops. There
is no preload phase, goroutine, clock, network access, power change, capture,
restoration, or automatic retry. The transition duration is independent of the
call frequency. Send the whole batch in order before advancing. If a send
fails, retain that batch for an explicit retry or stop; calling `next()` again
advances the sequence even if the earlier frame did not reach the device.

Sequences can contain more frames than there are device frame buffers. At most
64 colors produces one visible-buffer Set64. Larger frames reuse
`SetMatrixColorsFromSlice`: Set64 packets stage the complete frame in buffer 1,
followed by CopyFrameBuffer into visible buffer 0 with the requested transition.
Buffer 2 is not used. This is transient staging, not preloading one frame into
each hidden buffer. Coordinate buffer 1 use with other writers; neither staging
nor multi-packet sends are protected from another client.

All frames must have identical sizes and complete rows. Width and height must
fit 1..255. For multi-packet frames, width must divide 64 because the reused
builder chunks at 64 colors; unsupported layouts fail rather than silently
misalign rows. The tile range must fit uint8 fields without wrapping; duration
must fit nonnegative uint32 milliseconds. Total stored colors are limited to
1,048,576. These checks do not establish firmware/device support.

Inputs are raw protocol HSBK values in **physical packet order**, not percentages,
RGB, images, visible-zone lists, or logical display coordinates. All brightness
values, including black, are retained without additional scaling. A length
greater than one applies the same frame to each addressed chain device, as the
existing message builder does; differing chain frames require separate sequences.
The function does not map orientation or preserve an uplight slot.

For images, themes or logical effects, use existing surface/frame adapters and
device renderers to perform the geometry/orientation mapping. The effects runner
already supplies a timed logical-frame execution path. Image decoding and palette
extraction are outside this iterator. Caller-owned timers should be stopped on
cancellation; choose frequency based on packet count and observed network health.
Successful sends are not acknowledgements or proof of device application.

`SetMatrixFrameAnimation` is deprecated but retains its old behavior for existing
callers. It preloads one hidden buffer per frame; firmware exposing only buffers
0, 1 and 2 cannot support more than two such hidden frames. Its old preload /
single-copy callback contract cannot be silently changed to packet streaming.
