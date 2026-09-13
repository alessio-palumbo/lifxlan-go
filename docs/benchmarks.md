# Benchmarks

lifxlan-go includes repeatable benchmarks for the in-memory operations most
relevant to applications that inspect or subscribe to device state:

- cloning single-zone, multizone, matrix, and switch device snapshots;
- snapshotting controllers with realistic mixtures of 1 to 100 devices;
- cloned point lookups by serial;
- concurrent controller snapshots;
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
  -bench 'Benchmark(DeviceClone|ControllerGetDevice|ControllerGetDevices|ControllerGetDevicesParallel|SubscribeDevices|DeviceEventFanout|DeviceEventSSE)$' \
  -benchmem -benchtime=500ms -count=10 \
  ./pkg/device ./pkg/controller ./internal/httpapi
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
- `allocs/op`: allocation count per operation; and
- `wire-B/event`: the encoded size of one SSE event frame.

Lower values are better for each of these measurements. Throughput can be
derived from `ns/op`, but real applications should also account for their own
processing and network behavior.
