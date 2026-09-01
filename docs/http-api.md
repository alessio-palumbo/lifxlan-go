# HTTP API

`lifxland` embeds one long-lived `controller.Controller` and exposes its device
inventory and state controls to non-Go applications. The first milestone is
deliberately small: discovery, inventory, and light/relay state. It does not add
effects, events, persistence, MCP, or gRPC.

Run it locally:

```sh
go run ./cmd/lifxland
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/v1/devices
```

The default listener is `127.0.0.1:8080` and is reachable only by clients on the
same machine. A non-loopback listener is useful when `lifxland` runs on an
always-on Raspberry Pi, NAS, or home server and clients on other devices use it
as their LIFX transport layer.

A non-loopback listener requires a bearer token so the control API cannot be
used without authentication:

```sh
LIFXLAN_API_TOKEN='replace-me' go run ./cmd/lifxland -listen 0.0.0.0:8080
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

## Python

The dependency-free example in [`examples/http/python/client.py`](../examples/http/python/client.py)
lists devices and updates a selector. It uses only Python's standard library:

```sh
python3 examples/http/python/client.py
LIFX_SELECTOR='location_id:936c9ba4-9f6d-4f12-8a83-2bed1d07ae42' \
  python3 examples/http/python/client.py
```
