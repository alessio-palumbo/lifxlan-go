# lifxlan-go

`lifxlan-go` is a Go client library for discovering and controlling [LIFX](https://www.lifx.com) smart lights over your local network using the LIFX LAN protocol.

It provides everything needed to build local-first LIFX applications, including device discovery, protocol messaging, state tracking, and a natural-language command parser.

This library is designed to be lightweight, idiomatic, and suitable for CLI tools, desktop apps, automation services, and embedded controllers.

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

You can use Match() for autocomplete, suggestions, or fuzzy device selection in your UI or CLI application.

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
- pkg/device – contains Device definition and properties
- pkg/client – low-level UDP client for communicating with LIFX protocol
- pkg/protocol – contains the LIFX Message library
- pkg/messages – a selection of ready-to-use LIFX messages
- pkg/matrix – a library to perform matrix editing and effects
- pkg/command – simple natural-language → Command compiler

## Contributing

Issues, feature requests, and PRs are welcome!

## License

MIT
