package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"text/tabwriter"

	"github.com/alessio-palumbo/lifxlan-go/pkg/controller"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	ucli "github.com/urfave/cli/v3"
)

type lightPartBackend interface {
	GetLightPartState(device.Serial, device.LightPart) (controller.LightPartState, error)
	SetLightPartColor(context.Context, device.Serial, device.LightPart, controller.LightStateUpdate, controller.SnapshotOptions) error
	SetLightPartPower(context.Context, device.Serial, device.LightPart, bool, controller.LightPartPowerOptions) error
}

type lightPartView struct {
	device.LightPartInfo
	State controller.LightPartState
}

func runLightParts(ctx context.Context, name string, cmd *ucli.Command, out, diagnostic io.Writer, create factory) error {
	part := device.LightPart(cmd.String("part"))
	if name != "parts" && part != device.LightPartAll && part != device.LightPartMain && part != device.LightPartUplight {
		return errors.New("--part must be all, main, or uplight")
	}
	var update controller.LightStateUpdate
	update.Duration = cmd.Duration("duration")
	power := controller.LightPartPowerOptions{Duration: update.Duration, Timeout: cmd.Duration("timeout")}
	if name == "part power" {
		if cmd.Args().Len() != 1 || (cmd.Args().First() != "on" && cmd.Args().First() != "off") {
			return errors.New("devices power requires on or off; put flags before it")
		}
		if cmd.IsSet("fallback-brightness") {
			v := cmd.Float64("fallback-brightness")
			if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 || v > 100 {
				return errors.New("--fallback-brightness must be finite, >0 and <=100")
			}
			power.FallbackBrightness = &v
		}
		if update.Duration < 0 || update.Duration.Milliseconds() > math.MaxUint32 {
			return errors.New("invalid --duration")
		}
	} else if cmd.Args().Len() != 0 {
		return errors.New("unexpected arguments")
	}
	if name == "part color" {
		for flag, field := range map[string]**float64{"hue": &update.Hue, "saturation": &update.Saturation, "brightness": &update.Brightness} {
			if cmd.IsSet(flag) {
				v := cmd.Float64(flag)
				*field = &v
			}
		}
		if cmd.IsSet("kelvin") {
			v := cmd.Float64("kelvin")
			if math.IsNaN(v) || math.IsInf(v, 0) || v < 1 || v > math.MaxUint16 || v != math.Trunc(v) {
				return errors.New("--kelvin must be an integer between 1 and 65535 (device limits also apply)")
			}
			k := uint16(v)
			update.Kelvin = &k
		}
		if err := controller.ValidateStateUpdate(controller.StateUpdate{Light: &update}); err != nil {
			return err
		}
	}
	c, err := create(cmd.Bool("verbose"), diagnostic)
	if err != nil {
		return err
	}
	defer c.Close()
	b, ok := c.(lightPartBackend)
	if !ok {
		return errors.New("backend does not support light parts")
	}
	if err := waitForDiscovery(ctx, cmd.Duration("discover-for"), diagnostic, false); err != nil {
		return err
	}
	d, err := selectDevice(c.GetDevices(), cmd.String("target"))
	if err != nil {
		return err
	}
	if len(device.LightParts(d)) == 0 {
		return controller.ErrUnsupportedCapability
	}
	op, cancel := context.WithTimeout(ctx, cmd.Duration("timeout"))
	defer cancel()
	if name == "parts" {
		if cmd.Bool("fresh") {
			if _, err := c.CaptureStateSnapshot(op, []device.Serial{d.Serial}, controller.SnapshotOptions{Timeout: cmd.Duration("timeout"), RequireFresh: true}); err != nil {
				return err
			}
			d, err = selectDevice(c.GetDevices(), d.Serial.String())
			if err != nil {
				return err
			}
		}
		var views []lightPartView
		for _, info := range device.LightParts(d) {
			state, err := b.GetLightPartState(d.Serial, info.Part)
			if err != nil {
				return err
			}
			views = append(views, lightPartView{info, state})
		}
		if cmd.String("output") == "json" {
			return writeJSON(out, struct {
				Serial, Label string
				Parts         []lightPartView
			}{d.Serial.String(), d.Label, views})
		}
		if _, err := fmt.Fprintf(out, "%s (%s)\n", d.Label, d.Serial); err != nil {
			return err
		}
		w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "PART\tTYPE\tZONES\tON\tSTORED BRIGHTNESS\tDEVICE POWER\tEMULATED")
		for _, v := range views {
			zones, on, brightness, devicePower := "?", "?", "?", "?"
			if v.ZonesKnown {
				zones = fmt.Sprint(v.ZoneCount)
			}
			if v.State.Known {
				on = fmt.Sprint(v.State.On)
				brightness = fmt.Sprintf("%.2f%%", v.State.Brightness)
				devicePower = fmt.Sprint(v.State.DevicePoweredOn)
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%t\n", v.Part, v.LightType, zones, on, brightness, devicePower, v.State.Emulated)
		}
		return w.Flush()
	}
	if name == "part color" {
		err = b.SetLightPartColor(op, d.Serial, part, update, controller.SnapshotOptions{Timeout: cmd.Duration("timeout"), RequireFresh: true})
	} else {
		err = b.SetLightPartPower(op, d.Serial, part, cmd.Args().First() == "on", power)
	}
	if err != nil {
		return err
	}
	// A successful send is not an acknowledgement or a fresh state observation.
	result := struct {
		Serial string
		Part   device.LightPart
		Result string
	}{d.Serial.String(), part, "operation completed; use devices parts to verify observed state"}
	if cmd.String("output") == "json" {
		return writeJSON(out, result)
	}
	_, err = fmt.Fprintf(out, "%s (%s), %s: %s\n", d.Label, d.Serial, part, result.Result)
	return err
}
