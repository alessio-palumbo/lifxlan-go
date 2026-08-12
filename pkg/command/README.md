# pkg/command

`pkg/command` converts short user-facing text commands into LIFX protocol messages.
It does not send messages, manage devices, keep UI state, or remember previous commands.
Callers provide the current device list, parse text into `Command` values, then decide how to send the generated messages.

```go
parser := command.NewCommandParser(devices)
cmds := parser.Parse("desk warm white")

for _, cmd := range cmds {
    cmd.ForEachSend(func(serial device.Serial, msg *protocol.Message) {
        _ = ctrl.Send(serial, msg)
    })
}
```

## Matching Targets

The parser builds selectors from the provided devices:

- serial, such as `d073d561034a`
- label, such as `desk`
- group, such as `living room`
- location, such as `home`
- `all`

Selectors can appear before or after actions:

```text
desk blue
blue desk
living room brighter
```

Multiple commands can be separated with punctuation or `then`:

```text
desk blue then bedroom off
```

## Absolute Actions

Supported absolute actions include power, hue/color, brightness, saturation, kelvin, and duration.

```text
desk on
desk off
desk blue
desk brightness 30%
desk 4000k
desk saturation 50%
desk red in 2s
```

Named color words set hue and saturation. Styled color phrases set hue from the color word and saturation from the style word:

```text
desk pastel blue
desk soft pink brightness 40%
desk vivid green
desk deep purple
```

| Style word | Saturation |
| --- | ---: |
| `washed` | 25 |
| `pastel` | 35 |
| `soft` | 45 |
| `muted` | 50 |
| `deep` | 90 |
| `rich` | 90 |
| `vivid` | 100 |
| `strong` | 100 |

White-temperature words set saturation to `0` so Kelvin is visible:

| Command word | Saturation | Kelvin |
| --- | ---: | ---: |
| `white` | 0 | unchanged |
| `candlelight` | 0 | 2200 |
| `warm white` | 0 | 2700 |
| `soft white` | 0 | 3000 |
| `neutral white` | 0 | 4000 |
| `daylight` | 0 | 5600 |
| `cool white` | 0 | 6500 |

## Relative Actions

Relative actions are compiled using the current device state passed to `NewCommandParser`.
If one selector resolves to multiple devices, relative actions become one command per device so each target uses its own current state.

Brightness changes clamp to `[1, 100]` so a powered-on light is not left at zero brightness:

```text
desk dim
desk dim 20%
desk brighter
desk more bright
desk less bright
```

Saturation changes clamp to `[0, 100]`:

```text
desk softer
desk vivid
desk more vivid
desk less intense
desk more pastel
desk less pastel
```

Kelvin changes clamp to the target device temperature range when known, or `[1500, 9000]` otherwise:

```text
desk warmer
desk cooler
desk more warm
desk less cool
```

Relative Kelvin changes do not modify saturation. If a bulb is currently showing a saturated color, `warmer` and `cooler` may update state without a visible change. Use `warm white`, `cool white`, or another white-temperature word when the user intent is a visible white temperature.

## Current Limits

The parser intentionally handles compact commands, not conversational state. It does not infer a target from a previous command, so input such as `a bit warmer` needs an application-level selected target or command history before parsing.

Only `style + color` order is modeled for styled colors. For example, `pastel blue` is supported, while `blue pastel` is not.

## Autocomplete

Use `Match` to drive suggestions for device labels, groups, locations, and serials:

```go
matches := parser.Match("ki")
```
