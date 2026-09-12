"""Minimal standard-library client for the lifxland HTTP API."""

import json
import os
import urllib.request


BASE_URL = os.getenv("LIFXLAN_URL", "http://127.0.0.1:8080")
TOKEN = os.getenv("LIFXLAN_API_TOKEN")


def request(path: str, *, method: str = "GET", body: dict | None = None) -> dict:
    data = None if body is None else json.dumps(body).encode()
    headers = {"Accept": "application/json"}
    if body is not None:
        headers["Content-Type"] = "application/json"
    if TOKEN:
        headers["Authorization"] = f"Bearer {TOKEN}"
    req = urllib.request.Request(BASE_URL + path, data=data, headers=headers, method=method)
    with urllib.request.urlopen(req) as response:
        return json.load(response)


def device_events():
    """Yield decoded events from lifxland's SSE stream."""
    headers = {"Accept": "text/event-stream"}
    if TOKEN:
        headers["Authorization"] = f"Bearer {TOKEN}"
    req = urllib.request.Request(BASE_URL + "/v1/devices/events", headers=headers)
    with urllib.request.urlopen(req) as response:
        for raw_line in response:
            line = raw_line.decode("utf-8").rstrip("\r\n")
            if line.startswith("data:"):
                yield json.loads(line.removeprefix("data:").lstrip())


devices = request("/v1/devices")
print(json.dumps(devices, indent=2))

selector = os.getenv("LIFX_SELECTOR")
if selector:
    result = request(
        "/v1/devices/state",
        method="PATCH",
        body={
            "selectors": [selector],
            "light": {
                "power": "on",
                "color": {"brightness": 60},
                "duration_ms": 500,
            },
        },
    )
    print(json.dumps(result, indent=2))

if os.getenv("LIFX_WATCH"):
    for event in device_events():
        print(json.dumps(event, indent=2))
