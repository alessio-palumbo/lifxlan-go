# Read-only frame-buffer inspection

```sh
go run ./cmd/lifxlan devices buffers --target Ceiling
go run ./cmd/lifxlan devices buffers --target Ceiling --buffer 2 --output json
```

`Ceiling` is an example user-assigned label; replace it with your matrix device's
label or serial. The command resolves exactly one matrix device using normal discovery, then
uses the library's UDP client on a dedicated query socket to request physical
colours with `TileGet64`. The query socket first sends GetService and waits for
the target's UDP service reply, matching the controller's discovery-first
initialization rather than assuming a peer discovered on another socket is
immediately reachable. This initialization also uses `--timeout` as its bound.
No Set, CopyFrameBuffer, power, or effect commands are
sent. The normal controller remains responsible for discovery and its ordinary
state queries. Hidden replies never overwrite cached visible state.

By default it reads buffers 0, 1 and 2. `--buffer` can be repeated to select a
subset; duplicate values are ignored. These are candidate indices for this
diagnostic, not a guarantee of firmware support. `--discover-for` waits for
inventory/geometry; `--timeout` bounds observation of each entire buffer across
all matrix chains and packets. Invalid indexes fail before opening a controller.

Replies must match the source, sequence, serial, UDP endpoint, chain and requested
rectangle (including buffer index). Firmware returning buffer 0 for a hidden
request is reported as a mismatch, not labelled as hidden state. Timeouts and
send/receive errors are explicit. Any failed read makes the command exit nonzero
after printing available results. Cancellation interrupts the dedicated socket.

Text output includes per-buffer packet coverage, a SHA-256 of physical HSBK
colours, and each chain's nonblack count plus first/last cell. For the Ceiling 13"
(PID 265)
8×8 layout the last cell is physical index 63, the registry-designated uplight.
JSON includes complete `Chains` only when every requested packet was observed;
otherwise `Known` is false and `Chains` is null, never fabricated black state.
Valid received black buffers remain known. Reads are not atomic across packets
or buffers, and unsupported/missing geometry fails clearly.

Run before and after changing only uplight in the official app, then compare
hashes and physical cells. Changed hidden buffers show activity but do not prove
their purpose; unchanged buffers cannot exclude transient staging between reads.
Hidden-buffer contents do not establish ownership or validity of a soft-off
backup. This command does not change polling policy, allocate device buffers,
restore colours, or implement uplight soft-off.
