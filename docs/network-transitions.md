# Device and host network transitions

UDP StateService discovery identifies devices by serial. When a known serial
advertises a different endpoint, the controller updates its existing session
using the reply's source IP and advertised UDP port. Malformed endpoints and
non-UDP services are ignored. IP, port and IPv6 zone changes are tracked; the
address is copied rather than retained as caller-owned storage.

Identity, cached values and subscriptions survive migration. A single
`DeviceEventUpdated` with `DeviceChangeAddress` reports the new endpoint; there
is no artificial remove/add cycle. Duplicate same-endpoint announcements do not
emit updates. State queries refresh cached metadata/colours, while old observation
coverage is invalidated. Cached values remain available but are not newly observed
merely because an address changed. Snapshot capture waits for new coverage.

Packet batches retain one endpoint for their duration. Queued network replies
from an old endpoint generation and later replies from its old IP are ignored.
Pending pings return `ErrEndpointChanged` so callers may retry. Discovery is
unauthenticated UDP, not identity authentication: conflicting announcements can
still redirect/flap the endpoint. No automatic colour, power, or restore writes
are triggered by migration.

For a known host subnet change on the same selected interface:

```go
// After the host finishes moving from setup Wi-Fi to normal Wi-Fi:
if err := ctrl.RefreshDiscovery(); err != nil {
    // Retry after connectivity returns, or recreate the controller if needed.
    return err
}
```

This explicitly invokes the default client's `RefreshBroadcastTarget`, then sends
discovery once. It does not alter automatic discovery cadence or recreate sockets
or sessions. A configured interface name/index is honoured. Automatically selected
interfaces remain pinned to their original name; refresh does not switch silently
to Ethernet, VPNs or another interface. Exact configured BroadcastAddr values are
never replaced. Missing-interface errors retain the previous target; callers must
retry refresh when that interface returns. An intentional interface change needs
a new appropriately configured controller/client.

The Controller.Client interface is unchanged. Custom clients can optionally
implement `RefreshBroadcastTarget() error`; otherwise RefreshDiscovery returns
`ErrDiscoveryRefreshUnsupported`. This is subnet-discovery recovery, not a promise
to recover every OS socket failure, nor an mDNS interface lifecycle implementation.
