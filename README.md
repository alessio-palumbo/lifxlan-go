# lifxlan-go

`lifxlan-go` is a Go client library for discovering and controlling [LIFX](https://www.lifx.com) smart lights over your local network using the LIFX LAN protocol.

It provides everything needed to build local-first LIFX applications, including device discovery, protocol messaging, state tracking, and a natural-language command parser.

This library is designed to be lightweight, idiomatic, and suitable for CLI tools, desktop apps, automation services, and embedded controllers.

Repeatable performance benchmarks cover device snapshots, controller state
reads, subscription fan-out, and HTTP event encoding. Tagged releases publish
raw results and a same-runner comparison with the previous release; see the
[benchmark guide](docs/benchmarks.md) for scope and interpretation.

## Performance at a glance

An illustrative development run on an Apple M1 Pro with Go 1.26.7 produced the
following rounded results. Tagged-release reports from the consistent Linux CI
runner remain the canonical comparison between versions.

| Operation | Time | Allocated | Work |
| --- | ---: | ---: | ---: |
| Read one 5×64-zone matrix device | 0.9 µs | 3.3 KB | one cloned device |
| Snapshot 100 mixed devices | 79 µs | 171 KB | 100 cloned devices |
| Publish a matrix update to 10 subscribers | 7.6 µs | 30 KB | 10 snapshots |
| Encode and write one light SSE event | 1.4 µs | 1.0 KB | 502 wire bytes |
| Resolve and encode a color update for one light | 1.5 µs | 1.9 KB | one LAN packet |
| Resolve and encode a group color update for 50 lights | 96 µs | 142 KB | 50 LAN packets |
| Encode an 82-zone multizone update | 8.6 µs | 3.2 KB | one 700-byte packet |
| Encode a 128-zone matrix frame | 14 µs | 5.6 KB | three packets, 1,167 bytes |

These measure in-memory library and daemon work. They do not measure UDP
delivery, Wi-Fi conditions, device processing, transitions, or the time until a
light visibly changes. See the [benchmark guide](docs/benchmarks.md) and tagged
[GitHub Releases](https://github.com/alessio-palumbo/lifxlan-go/releases) for
the full statistical results and environment details.

## Features

- Discover LIFX devices via UDP broadcast
- Send and receive messages using the LIFX LAN protocol
- Manage per-device sessions
- Track device state (power, color, label, etc.)
- Perform periodic discovery and session health checks
- Natural-language command parsing → protocol messages
- Fully testable and modular architecture
- Extensible for advanced control

## Installation

```bash
go get github.com/alessio-palumbo/lifxlan-go
```

## Usage

```go
import (
	"fmt"
	"log"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/controller"
)

func main() {
	ctrl, err := controller.New()
	if err != nil {
		log.Fatal(err)
	}
	defer ctrl.Close()

	time.Sleep(time.Second)

	devices := ctrl.GetDevices()
	for _, d := range devices {
		fmt.Printf("Found device: %s - %s (PoweredOn: %t)\n", d.Serial, d.Label, d.PoweredOn)
	}
}
```

For a point lookup, `GetDevice(serial)` returns one independent device snapshot
without cloning the complete inventory.

The controller is silent by default.
To receive controller and device-session logs, pass a standard `log/slog` logger:

```go
logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
ctrl, err := controller.New(controller.WithLogger(logger))
```

## HTTP API

Applications written in Python or other languages can use the bundled
`lifxland` daemon as their LIFX LAN transport layer:

```sh
./lifxland
curl http://127.0.0.1:8080/v1/devices
```

Prebuilt archives are published on the
[GitHub Releases](https://github.com/alessio-palumbo/lifxlan-go/releases) page:

| Platform | Archive target |
| --- | --- |
| macOS, Apple Silicon | `darwin_arm64` |
| macOS, Intel | `darwin_amd64` |
| Linux, x86-64 | `linux_amd64` |
| Linux or Raspberry Pi OS, 64-bit ARM | `linux_arm64` |
| Raspberry Pi OS, 32-bit ARM | `linux_armv7` |
| Windows, x86-64 | `windows_amd64` |

Download the archive for your platform, verify it against `SHA256SUMS`, extract
it, and run `lifxland` (`lifxland.exe` on Windows). No Go installation is
required. To build the daemon from source instead, use `go run ./cmd/lifxland`.

The device-centric API supports lights, switches, and hybrid devices. State
updates use selector arrays and capability-scoped `light` and `relays` objects
at `PATCH /v1/devices/state`. Observed device changes are available as
Server-Sent Events at `GET /v1/devices/events`. See the
[HTTP API guide](docs/http-api.md) and [OpenAPI contract](api/openapi.yaml).

### Running `lifxland` on another machine

The default loopback listener is intended for clients running on the same
machine as `lifxland`. A non-loopback listener is useful when the daemon runs on
an always-on Raspberry Pi, NAS, or home server and applications on other
computers or phones use it as their LIFX transport layer.

Non-loopback listeners require `LIFXLAN_API_TOKEN`. The server operator chooses
this pre-shared token and configures the same value in every authorized client:

```sh
# Run on the server. Generate and store a suitably random value for real use.
LIFXLAN_API_TOKEN='replace-me' ./lifxland -listen 0.0.0.0:8080

# Run from an authorized client.
curl -H 'Authorization: Bearer replace-me' http://server-address:8080/v1/devices
```

The token authenticates HTTP requests; it does **not** encrypt them. Plain HTTP
can expose the token and request data to interception and replay. For access
outside a trusted network, use HTTPS through a reverse proxy, a VPN such as
WireGuard or Tailscale, or a secure tunnel. Keep the default loopback listener
when all clients run locally.

## Target Selection

`pkg/device` includes small selector helpers for apps that let users refer to
devices by serial, label, group, location, or `all`.

```go
devices := ctrl.GetDevices()
serials := device.ResolveSelectorSerials("desk, office", devices)

for _, serial := range serials {
	err := ctrl.Send(serial, messages.SetPowerOn())
	if err != nil {
		return err
	}
}
```

Selectors are comma-separated, case-insensitive exact matches. Results preserve
selector order, preserve device discovery order inside each selector, and
de-duplicate serials.

## Device State Events

Applications can subscribe to changes in the controller's cached device state
instead of repeatedly comparing complete device lists:

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

devices := make(map[device.Serial]device.Device)
for event := range ctrl.SubscribeDevices(ctx) {
	switch event.Type {
	case controller.DeviceEventAdded, controller.DeviceEventUpdated:
		devices[event.Device.Serial] = event.Device
	case controller.DeviceEventRemoved:
		delete(devices, event.Device.Serial)
	case controller.DeviceEventSnapshotComplete:
		fmt.Printf("initial inventory ready at revision %d\n", event.Revision)
	case controller.DeviceEventResyncRequired:
		devices = make(map[device.Serial]device.Device)
		for _, d := range ctrl.GetDevices() {
			devices[d.Serial] = d
		}
	}
}
```

Existing devices are emitted as initial `DeviceEventAdded` events at revision
zero, followed by `DeviceEventSnapshotComplete` carrying the baseline revision.
The completion marker is emitted even when the initial inventory is empty, and
subsequent live events have greater revisions. Each event contains an
independent device snapshot and update events identify the changed capability
or metadata category.

`DeviceEventAdded` means that discovery created a device session; it does not
guarantee that the preflight handshake has populated all capability-specific
state. Consumers should expect subsequent update events. Slow subscribers do
not block LAN message processing. If a subscriber exceeds its configured
pending-event buffer, it receives
`DeviceEventResyncRequired` and can recover using `GetDevices`.

Events describe state observed by the controller, not command acknowledgements.
The controller still polls devices according to its configured high- and
low-frequency refresh periods.
Time-based event coalescing is intentionally left to consumers so UIs can batch
renders while automation and monitoring clients can retain low latency.

Matrix and multizone device snapshots include their last observed firmware
effect. `Effect.Known` distinguishes a confirmed `off` state from an effect that
has not been queried yet, and `Effect.Running()` provides the common active-state
check. Effect setters request their corresponding state response, while periodic
high-frequency queries also detect effects started by another controller.

Effect instance IDs can be retained to correlate observed state with an effect
started by the application without depending on generated packet types:

```go
msg := messages.SetMatrixFlameEffect(5 * time.Second)
instanceID, _ := messages.EffectInstanceID(msg)
// Record instanceID before Send because the state response may arrive promptly.
err := ctrl.Send(serial, msg)
```

An instance ID is a correlation token rather than proof of ownership; another
controller can replace the effect at any time.

A runnable [Go device monitor](examples/monitor/main.go) demonstrates the full
subscription lifecycle while maintaining and printing a compact device
inventory:

```sh
go run ./examples/monitor
```

It displays device identity, labels, location, group, product information,
device type, power, color, and the cached zone count for multizone and matrix
lights. Event bursts are coalesced, and an interactive terminal is repainted in
place rather than appending a new table for every revision.

## State Snapshot And Restore

Controllers can capture and restore the current light state for one or more
devices. This is useful when an app runs a temporary scene, effect, or
choreography and wants to put lights back afterwards.

```go
ctx := context.Background()
serials := device.ResolveSelectorSerials("desk, office", ctrl.GetDevices())

snapshot, err := ctrl.CaptureStateSnapshot(ctx, serials, controller.SnapshotOptions{
	Timeout: 3 * time.Second,
})
if err != nil {
	return err
}

defer ctrl.RestoreStateSnapshot(context.Background(), snapshot, controller.RestoreOptions{
	Duration: 500 * time.Millisecond,
	Attempts: 2,
})
```

Snapshot capture requests the state needed for each light type: power and color
for single-zone lights, zone colors for multizone lights, and matrix chain colors
for matrix lights. Restore replays the matching protocol messages later.

The library handles the LIFX-specific state shape, while applications still own
policy decisions such as when a snapshot is stale, how long to wait before
starting an effect, and whether to retry restoration.

## Color Helpers

`device.Color` stores hue, saturation, and brightness as user-facing
percentages/degrees. Helpers are available for common brightness rules:

```go
device.ClampBrightness(value)        // clamp to [0, 100]
device.ClampVisibleBrightness(value) // clamp to [1, 100]
device.ScaleBrightness(value, 0.5)   // scale and keep a visible brightness
```

## Effects

The `pkg/effects` package generates deterministic, target-free frames that can be used live or rendered offline.
Frames do not contain serials, groups, labels, or network commands; device targeting is handled by renderers.

The shared rendering pipeline is:

```text
Effect -> Frame -> device.Surface -> DeviceFrame -> renderer/messages

physical HSBK state + device.Surface -> normalized logical/display Frame
```

`device.SurfaceFromDevice` derives logical/display layout and physical send metadata from a discovered device.
This includes multizone sizing, matrix chain bounds, send width, row offsets, hidden cells, and matrix orientation.

### Run Effects Live

```go
import (
	"context"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/controller"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/effects"
	"github.com/alessio-palumbo/lifxlan-go/pkg/effects/adapters"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
)

func runSweep(ctx context.Context, ctrl *controller.Controller, dev device.Device) error {
	send := func(msg *protocol.Message) error {
		return ctrl.Send(dev.Serial, msg)
	}

	caps := effects.CapabilitiesFromDevice(dev)
	palette := effects.Palette{
		Base:        []effects.Color{{Hue: 210, Saturation: 100, Brightness: 35, Kelvin: 3500}},
		Accents:     []effects.Color{{Hue: 25, Saturation: 100, Brightness: 60, Kelvin: 3000}},
		Backgrounds: []effects.Color{{Hue: 260, Saturation: 85, Brightness: 8, Kelvin: 3500}},
	}

	return adapters.RunEffects(ctx, dev, send,
		effects.RunConfig{
			Effect: effects.NewSweep(effects.SweepConfig{
				Capabilities: caps,
				Palette:      palette,
			}),
			Duration: 10 * time.Second,
			Step:     120 * time.Millisecond,
		},
	)
}
```

`adapters.RunEffects` configures the right renderer from the discovered device:

- single-zone lights use color messages
- multizone lights adapt frames to the device surface and use extended zone color messages
- matrix lights adapt frames to the device surface, preserve send width/layout, and apply device orientation when sending tile color messages

For lower-level control, build a renderer yourself:

```go
renderer := adapters.NewRendererForDevice(dev, send)
runner := effects.NewRunner(effect, renderer, 120*time.Millisecond)
err := runner.Run(ctx)
```

You can also pass a known surface to lower-level renderers:

```go
surface := device.SurfaceFromDevice(dev)
renderer := adapters.NewMatrixRenderer(send, adapters.WithMatrixSurface(surface))
```

### Render Offline

Use `effects.Render` when you need timestamped frames without touching the network.
This is useful for tests, previews, or offline choreography generation.

```go
frames := effects.Render(
	effects.NewGradient(effects.GradientConfig{
		Capabilities: effects.Capabilities{Width: 8, Height: 8},
		Palette:      palette,
	}),
	100*time.Millisecond,
	10*time.Second,
)
```

To convert a logical frame into packet-independent device frames, adapt it to a surface:

```go
surface := device.SurfaceFromDevice(dev)
deviceFrames, err := effects.AdaptFrameToSurface(frames[0].Frame, surface, effects.AdaptOptions{})
```

The resulting `DeviceFrame` values contain colors, duration, send width, chain index, and orientation metadata.
They can be serialized into a timeline, rendered in a preview, or converted to LAN messages later.

### Adapt Physical State for Previews

`effects.PhysicalColorState` keeps exact HSBK values in device packet order.
It can merge partial single-zone, multizone, or matrix updates before adapting
the accumulated state back to a logical frame. For example, a received
multizone range can update an existing preview without rebuilding unrelated
zones:

```go
surface := device.Surface{
	LightType: device.LightTypeMultiZone,
	Width:     16,
	Height:    1,
	Zones:     16,
}
physical := effects.NewPhysicalColorState(surface)

// Preserve the packet's exact HSBK values. For an extended multizone packet,
// use only Colors[:ColorsCount].
err := physical.MergeZoneColors(startIndex, receivedColors)
if err != nil {
	return err
}

preview, err := effects.AdaptPhysicalColorStateToFrame(physical, surface, 0)
```

For matrix state, use `MergeMatrixColors` with the chain index and physical
`x`, `y`, and send width. Adaptation removes device orientation, positions each
chain at its logical bounds, applies row offsets, and leaves hidden or
non-emitting cells blank. Returned frames own their color slices and do not
alias the physical state.

Available effects include `Solid`, `Gradient`, `GradientDrift`, `PaletteSweep`, `Comet`, `Sparkle`, `Sweep`, `Flow`, `Ring`, `Waterfall`, `Rockets`, `Snake`, `Worm`, `Wave`, and `ConcentricFrames`.

`Flow` defaults to a moving brightness crest. For filled matrix-style color
motion where palette brightness should stay constant, use:

```go
flow := effects.NewFlow(effects.FlowConfig{
	Capabilities:   caps,
	Palette:        palette,
	Axis:           effects.FlowAxisDiagonal,
	BrightnessMode: effects.FlowBrightnessConstant,
	Sampling:       effects.FlowSamplingInterpolate,
})
```

`Flow` and `GradientDrift` default to whole-cell palette steps. Use
`FlowSamplingInterpolate` when slower live effects should blend between palette
stops instead of holding each zone offset until the next step.

`PaletteSweep` moves a multi-color band over a dim drifting gradient background.
It is useful for strip and matrix choreography where the whole surface should stay
lit while a stronger palette band travels across it.

The older `pkg/matrix` effect helpers are kept for compatibility, but new code
should prefer `pkg/effects` plus `pkg/effects/adapters`. The newer API separates
deterministic frame generation from live LAN rendering and also supports offline
timeline generation.

## 🛠️ Creating Custom LIFX Messages

The messages package provides helpers to build your own LAN messages using the lifxprotocol-go types.

```go
import (
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

var SetColor = protocol.NewMessage(&packets.LightSetColor{
	Color:    packets.LightHsbk{Hue: 65535, Saturation: 65535, Brightness: 32768, Kelvin: 3500},
	Duration: 1000,
})
```

Then you can send it using the controller:

```go
err = controller.Send(deviceAddr, msg)
```

## 🌐 Multi-Network Discovery

By default, lifxlan-go keeps its existing automatic behavior and broadcasts on the first suitable IPv4 interface.
For machines connected to multiple networks, applications can list broadcast-capable interfaces and let users choose one.

```go
ifaces, err := client.BroadcastInterfaces()
if err != nil {
	panic(err)
}
for _, iface := range ifaces {
	fmt.Printf("%s %s -> %s\n", iface.Name, iface.IP, iface.Broadcast)
}
```

Use no option for automatic selection, or pass a selected interface when creating the controller.

```go
ctrl, err := controller.New(controller.WithClientConfig(&client.Config{
	BroadcastInterfaceName: "en0",
}))
```

The resolved interface is available for diagnostics in both automatic and
configured-interface modes:

```go
if iface, ok := ctrl.BroadcastInterface(); ok {
	fmt.Printf("using %s: %s -> %s\n", iface.Name, iface.IP, iface.Broadcast)
}
```

Automatic selection with no suitable interface returns
`client.ErrNoBroadcastInterface`. A configured name or index that is no longer
available returns `*client.BroadcastInterfaceNotFoundError`, so applications
can use `errors.Is` and `errors.As` instead of matching error text. Socket bind,
send, and receive failures retain their standard Go network error types.

Advanced callers can also provide an exact broadcast address. If the port is zero, the default LIFX UDP port is used.

```go
c, err := client.NewClient(&client.Config{
	BroadcastAddr: &net.UDPAddr{IP: net.IPv4(192, 168, 1, 255)},
})
```

`BroadcastInterface` returns `false` for an exact `BroadcastAddr` override
because no OS interface was resolved. Interface-name selection continues to
use the first suitable IPv4 address on that interface; callers that require a
specific subnet can use an exact broadcast address.

If an application changes the selected interface at runtime, close the current controller and create a new one so discovery and device sessions are rebuilt for the selected network.

## 🔧 Using the Client Directly

If you prefer low-level control or want to use your own device management logic, you can use the Client directly without the higher-level Controller.

This is ideal for:

- Quick testing
- One-off commands
- Custom applications that don’t need device sessions or periodic discovery

```go
import (
	"fmt"
	"net"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/client"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func main() {
	c, err := client.NewClient(nil)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	done := make(chan struct{})
	go c.Receive(2*time.Second, false, func(m *protocol.Message, addr *net.UDPAddr) {
		fmt.Printf("Received: %+v from %v\n", m.Target(), addr)
		close(done)
	})

	msg := protocol.NewMessage(&packets.DeviceGetService{})
	err = c.SendBroadcast(msg)
	if err != nil {
		panic(err)
	}

	<-done
}
```

You can:

- Use client.Send() or client.SendBroadcast() to send commands.
- Start a background client.Receive() to process incoming messages.
- Build and customize your own logic for managing responses.

## 🧠 Command Parsing

The command parser converts user text into executable protocol messages.
See [`pkg/command/README.md`](pkg/command/README.md) for the supported grammar and current limitations.
This allows applications to support natural commands like:

```
set kitchen lights orange 50%
desk lamp off
bedroom lights blue and dim 20%
```

Example:

```go
parser := commandparser.NewCommandParser(devices)
cmds := parser.Parse("kitchen lights warm white 50%")

for _, cmd := range cmds {
    cmd.ForEachSend(func(s device.Serial, msg *protocol.Message) {
		_ = ctrl.Send(s, msg)
    })
}
```

### Matching and autocomplete

The parser also supports matching device names, groups, or locations based on partial or fuzzy input using:

```go
matches := parser.Match("ki") // returns top matches for "ki", e.g. ["kitchen lights", "kit lamp"]
```

You can use Match(term) for autocomplete, suggestions, or fuzzy device selection in your UI or CLI application.

## 📦 Dependencies

This package depends on:

- (lifxprotocol-go)[github.com/alessio-palumbo/lifxprotocol-go]: provides the generated protocol structs and enums.
- (lifxregistry-go)[github.com/alessio-palumbo/lifxregistry-go]: provides products information through the generated LIFX public registry.

Add it to your project:

```bash
go get github.com/alessio-palumbo/lifxprotocol-go
go get github.com/alessio-palumbo/lifxregistry-go
```

## Environment Variables

LIFX_LOG_LEVEL: Set the log level (info, debug, warn, error). Default is info.

## Project Structure

- pkg/controller – high-level controller for managing sessions and device state
- pkg/device – contains Device definition, properties, and surface/layout metadata
- pkg/client – low-level UDP client for communicating with LIFX protocol
- pkg/protocol – contains the LIFX Message library
- pkg/messages – a selection of ready-to-use LIFX messages
- pkg/effects – deterministic frame effects, live runners, and LIFX render adapters
- pkg/matrix – legacy matrix editing and blocking effect helpers; prefer pkg/effects for new code
- pkg/command – simple natural-language → Command compiler

## Contributing

Issues, feature requests, and PRs are welcome!

## License

MIT
