# Benchmarks

lifxlan-go includes repeatable benchmarks for the in-memory operations most
relevant to applications that inspect, control, or subscribe to device state:

- cloning single-zone, multizone, matrix, and switch device snapshots;
- snapshotting controllers with realistic mixtures of 1 to 100 devices;
- cloned point lookups by serial;
- concurrent controller snapshots;
- resolving a group and preparing encoded color updates for 1, 10, or 50 lights;
- preparing and encoding an 82-zone multizone color batch;
- preparing and encoding a 128-zone matrix frame with its framebuffer swap;
- initializing subscriptions with 0 to 100 existing devices;
- publishing an update to zero, one, five, or ten subscribers; and
- projecting, encoding, and writing HTTP Server-Sent Events.

These benchmarks deliberately exclude discovery and physical-device response
times. Those depend on the LAN, Wi-Fi conditions, polling configuration, and
device firmware, so a hosted CI result would not represent a user's network.

## Published release results

Tagged releases include two benchmark assets:

- `lifxlan-go_<version>_benchmarks.md` contains the environment and a
  statistical comparison with the previous reachable release tag.
- `lifxlan-go_<version>_benchmarks.txt` contains the raw Go benchmark samples.

The release workflow runs both versions on the same GitHub-hosted runner with
the same Go toolchain. This reduces environmental differences, but does not
eliminate hosted-runner noise. Treat allocation changes and large,
statistically significant timing changes as stronger signals than small timing
deltas. Benchmark assets describe library and daemon overhead, not end-to-end
command latency to a light.

Benchmarks run only for `v*` tag pushes and manually dispatched release builds.
Normal commits and pull requests continue to run correctness, race, vet, and
static-analysis checks without the benchmark workload.

## Running locally

Run the published suite:

```sh
go test -run '^$' \
  -bench 'Benchmark(DeviceClone|ControllerGetDevice|ControllerGetDevices|ControllerGetDevicesParallel|SetColorByGroup|SubscribeDevices|DeviceEventFanout|DeviceEventSSE|MultizoneColorBatch|MatrixColorBatch)$' \
  -benchmem -benchtime=500ms -count=10 \
  ./pkg/device ./pkg/controller ./pkg/messages ./internal/httpapi
```

To assess a change, collect the same command on the base and changed revisions,
then compare the files with `benchstat`:

```sh
benchstat base.txt changed.txt
```

Run comparisons on an otherwise idle machine, using the same power mode, Go
version, and benchmark flags for both revisions. CPU and memory profiles are
more useful than additional benchmark repetitions when investigating the cause
of a regression.

The primary measurements are:

- `ns/op`: elapsed time per operation;
- `B/op`: bytes allocated per operation;
- `allocs/op`: allocation count per operation;
- `devices/op` and `packets/op`: devices targeted and LAN packets prepared;
- `wire-B/op`: total encoded LIFX LAN bytes prepared by a control operation; and
- `wire-B/event`: the encoded size of one SSE event frame.

Lower values are better for timing and allocation measurements. Device, packet,
and wire-byte counts describe the work represented by an operation rather than
scores to minimize. Throughput can be derived from `ns/op`, but real
applications should also account for their own processing and network behavior.

Control benchmarks stop after messages have been selected, constructed,
encoded, and handed to an in-memory sender. They do not measure UDP delivery,
device processing, transitions, or the time until lights visibly change.
