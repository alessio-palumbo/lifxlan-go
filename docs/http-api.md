# HTTP API

`lifxland` embeds one long-lived `controller.Controller` and exposes its device
inventory and state controls to non-Go applications. The first milestone is
deliberately small: discovery, inventory, observed device events, and
light/relay state. It does not add effects, persistence, MCP, or gRPC.

Download the archive for your platform from
[GitHub Releases](https://github.com/alessio-palumbo/lifxlan-go/releases), verify
it against the accompanying `SHA256SUMS`, and extract it. Then run the daemon
locally:

```sh
./lifxland
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/v1/devices
```

No Go installation is required for a released binary. Developers building from
source can use `go run ./cmd/lifxland` instead. `./lifxland --version` prints the
embedded release tag.

Release archive targets are:

| Platform | Archive target |
| --- | --- |
| macOS, Apple Silicon | `darwin_arm64` |
| macOS, Intel | `darwin_amd64` |
| Linux, x86-64 | `linux_amd64` |
| Linux or Raspberry Pi OS, 64-bit ARM | `linux_arm64` |
| Raspberry Pi OS, 32-bit ARM | `linux_armv7` |
| Windows, x86-64 | `windows_amd64` |

For example, on a 64-bit Raspberry Pi after downloading the release archive:

```sh
tar -xzf lifxland_v0.8.0_linux_arm64.tar.gz
cd lifxland_v0.8.0_linux_arm64
./lifxland --version
./lifxland
```

The macOS binaries are currently unsigned and not notarized. After verifying
the checksum, macOS may require the binary to be approved in Privacy & Security
before it can run.

The default listener is `127.0.0.1:8080` and is reachable only by clients on the
same machine. A non-loopback listener is useful when `lifxland` runs on an
always-on Raspberry Pi, NAS, or home server and clients on other devices use it
as their LIFX transport layer.

A non-loopback listener requires a bearer token so the control API cannot be
used without authentication:

```sh
LIFXLAN_API_TOKEN='replace-me' ./lifxland -listen 0.0.0.0:8080
curl -H 'Authorization: Bearer replace-me' http://host:8080/v1/devices
```

The person operating the server chooses this pre-shared token and securely
provides the same value to authorized clients. Clients send it in the
`Authorization` header on each API request.

The token authenticates requests but does not encrypt traffic. With plain HTTP,
someone able to inspect the connection could capture the token and replay it.
Use an HTTPS reverse proxy, a VPN such as WireGuard or Tailscale, or a secure
tunnel when the network is not fully trusted. HTTPS or the tunnel protects the
HTTP connection between clients and `lifxland`; it does not alter the separate
LIFX LAN protocol connection between the daemon and the devices.

`GET /healthz` does not require authentication. The API contract is in
[`api/openapi.yaml`](../api/openapi.yaml).

## Selectors

`PATCH /v1/devices/state` accepts selector tokens as a JSON array. Arrays avoid
ambiguity when labels contain commas. Typed selectors are case-insensitive exact
matches:

- `all`
- `serial:<12 hex characters>`
- `label:<label>`
- `group:<label>` and `location:<label>`
- `group_id:<uuid>` and `location_id:<uuid>`

Bare selectors remain supported for compatibility and match serial, label,
group label, or location label. Prefer typed selectors in persisted clients,
especially UUID selectors: labels can be renamed or duplicated.

## State updates

Light power and color share one state object. When supplied together, the
controller sends color before power so sequencing is consistent for HTTP and Go
consumers. Hybrid devices can accept both `light` and `relays` in one request.

```sh
curl -X PATCH http://127.0.0.1:8080/v1/devices/state \
  -H 'Content-Type: application/json' \
  -d '{
    "selectors": ["group_id:936c9ba4-9f6d-4f12-8a83-2bed1d07ae42"],
    "light": {
      "power": "on",
      "color": {
        "hue": 210,
        "saturation": 80,
        "brightness": 60,
        "kelvin": 3500
      },
      "duration_ms": 500
    }
  }'
```

Relay state uses the same endpoint:

```sh
curl -X PATCH http://127.0.0.1:8080/v1/devices/state \
  -H 'Content-Type: application/json' \
  -d '{
    "selectors": ["serial:d073d5000001"],
    "relays": [{"index": 0, "power": "on"}]
  }'
```

Every matched device has an `accepted`, `partial`, or `rejected` result.
Unsupported capabilities are explicit rejections rather than silent skips. LIFX
LAN sends are not transactional: `partial` means one or more earlier messages
were sent before a later transport failure.

## Device events

`GET /v1/devices/events` is a
[Server-Sent Events](https://html.spec.whatwg.org/multipage/server-sent-events.html)
stream of the controller's observed device state:

```sh
curl -N http://127.0.0.1:8080/v1/devices/events
```

Authenticated remote clients send the same bearer token as other API requests:

```sh
curl -N -H 'Authorization: Bearer replace-me' \
  http://host:8080/v1/devices/events
```

Each frame has an event name of `added`, `updated`, `removed`, or
`resync_required`. Its `data` field is JSON:

```text
id: 14
event: updated
data: {"type":"updated","revision":14,"changes":["light"],"device":{...}}
```

When a connection starts, every currently active device is sent as an `added`
event with `initial: true`. Later updates include change categories and a
complete snapshot using the same representation as `GET /v1/devices/{serial}`.
The current HTTP device representation does not include detailed matrix pixel
buffers, multizone buffers, or button configuration.

Events describe changes observed in the controller's cache. LIFX state is still
polled on the LAN, and a successful state-changing request is not itself an
observation event. Revisions are monotonic for the lifetime of the daemon but
are not persisted or replayed.

Streaming never blocks device packet processing. If a client cannot consume
events quickly enough, it receives `resync_required` and must replace its local
view with `GET /v1/devices`. Reconnecting creates a fresh subscription and
again sends the current devices as initial events.

## Python

The dependency-free example in [`examples/http/python/client.py`](../examples/http/python/client.py)
lists devices and updates a selector. It uses only Python's standard library:

```sh
python3 examples/http/python/client.py
LIFX_SELECTOR='location_id:936c9ba4-9f6d-4f12-8a83-2bed1d07ae42' \
  python3 examples/http/python/client.py

LIFX_WATCH=1 python3 examples/http/python/client.py
```
