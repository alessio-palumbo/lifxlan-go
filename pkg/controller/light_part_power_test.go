package controller

import (
	"context"
	"errors"
	"math"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func powerDevice(main, up float64, on bool) device.Device {
	d := partDevice(265, 8, 8)
	d.PoweredOn = on
	for i := range d.MatrixProperties.ChainZones[0] {
		d.MatrixProperties.ChainZones[0][i].Brightness = device.ConvertExternalToDeviceValue(main, 100)
	}
	d.MatrixProperties.ChainZones[0][63].Brightness = device.ConvertExternalToDeviceValue(up, 100)
	return d
}

func frameInPlan(t *testing.T, plan partPowerPlan) []packets.LightHsbk {
	t.Helper()
	for _, m := range plan.messages {
		if p, ok := m.Payload.(*packets.TileSet64); ok {
			return p.Colors[:]
		}
	}
	return nil
}

func TestPartPowerPolicyUniformInheritanceAndLastOff(t *testing.T) {
	d := powerDevice(40, 50, true)
	before := d.Clone()
	plan, err := planPartPower(d, device.LightPartUplight, false, LightPartPowerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	colors := frameInPlan(t, plan)
	if plan.powerOff || colors[63].Brightness != 0 || colors[2] != d.MatrixProperties.ChainZones[0][2] {
		t.Fatal("uplight off changed main/power")
	}
	d.MatrixProperties.ChainZones[0] = append([]packets.LightHsbk(nil), colors...)
	plan, err = planPartPower(d, device.LightPartUplight, true, LightPartPowerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := frameInPlan(t, plan)[63].Brightness; got != device.ConvertExternalToDeviceValue(40, 100) {
		t.Fatal(got)
	}
	plan, err = planPartPower(d, device.LightPartMain, false, LightPartPowerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.powerOff || plan.storeUplight == nil || math.Abs(*plan.storeUplight-40) > 1e-6 || len(plan.messages) != 0 {
		t.Fatal(plan)
	}
	d = before.Clone()
	d.PoweredOn = false
	plan, err = planPartPower(d, device.LightPartUplight, true, LightPartPowerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	colors = frameInPlan(t, plan)
	if colors[2].Brightness != 0 || colors[63] != d.MatrixProperties.ChainZones[0][63] || plan.messages[len(plan.messages)-1].Type() != 21 {
		t.Fatal("part-only on failed to prepare before power")
	}
	if !reflect.DeepEqual(before, powerDevice(40, 50, true)) {
		t.Fatal("planner mutated inputs")
	}
}

func TestPartPowerAveragesVisibleCellsAndPreservesPatterns(t *testing.T) {
	d := powerDevice(0, 0, true)
	mask, err := lightPartMask(d, device.LightPartMain)
	if err != nil {
		t.Fatal(err)
	}
	index := 0
	for i, m := range mask {
		if m.Brightness == 0 {
			d.MatrixProperties.ChainZones[0][i].Brightness = 65535
			continue
		}
		b := 0.0
		if index%2 == 1 {
			b = 40
		}
		index++
		d.MatrixProperties.ChainZones[0][i].Brightness = device.ConvertExternalToDeviceValue(b, 100)
	}
	d.MatrixProperties.ChainZones[0][63].Brightness = 0
	plan, err := planPartPower(d, device.LightPartMain, false, LightPartPowerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if plan.storeUplight == nil || math.Abs(*plan.storeUplight-20) > 1e-6 {
		t.Fatalf("mean included hidden cells or excluded black: %+v", plan)
	}
	d.PoweredOn = false
	plan, err = planPartPower(d, device.LightPartMain, true, LightPartPowerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.messages) != 1 || plan.messages[0].Type() != 21 {
		t.Fatal("retained main pattern was flattened")
	}
	for _, part := range []device.LightPart{device.LightPartMain, device.LightPartUplight, device.LightPartAll} {
		black := powerDevice(0, 0, false)
		if _, err := planPartPower(black, part, true, LightPartPowerOptions{}); !errors.Is(err, ErrNoInheritedBrightness) {
			t.Fatal(err)
		}
		b := 1.0
		plan, err := planPartPower(black, part, true, LightPartPowerOptions{FallbackBrightness: &b})
		if err != nil {
			t.Fatal(err)
		}
		if frameInPlan(t, plan) == nil || plan.messages[len(plan.messages)-1].Type() != 21 {
			t.Fatal("fallback not prepared before on")
		}
	}
}

type powerTestSender struct {
	s             *deviceSession
	controls      []uint16
	ignoreOff     bool
	failColor     bool
	offConfirmed  bool
	stagedWhileOn bool
}

func (p *powerTestSender) Send(_ *net.UDPAddr, msg *protocol.Message) error {
	p.s.mu.Lock()
	defer p.s.mu.Unlock()
	s := p.s
	switch m := msg.Payload.(type) {
	case *packets.LightGet:
		s.observations.observe(s.device, &packets.LightState{})
		if !s.device.PoweredOn {
			p.offConfirmed = true
		}
	case *packets.TileGet64:
		s.observations.observe(s.device, &packets.TileState64{TileIndex: m.TileIndex, Rect: m.Rect})
	case *packets.DeviceSetPower:
		p.controls = append(p.controls, msg.Type())
		if m.Level != 0 {
			s.device.PoweredOn = true
		} else if !p.ignoreOff {
			s.device.PoweredOn = false
		}
	case *packets.LightSetPower:
		p.controls = append(p.controls, msg.Type())
		if m.Level != 0 {
			s.device.PoweredOn = true
		} else if !p.ignoreOff {
			s.device.PoweredOn = false
		}
	case *packets.TileSet64:
		if p.failColor {
			return errors.New("color send failed")
		}
		p.controls = append(p.controls, msg.Type())
		if s.device.PoweredOn {
			p.stagedWhileOn = true
		}
		copy(s.device.MatrixProperties.ChainZones[0], m.Colors[:])
	}
	return nil
}

func powerController(d device.Device) (*Controller, *powerTestSender) {
	c := newSnapshotController(newMockClient(), d)
	s := c.sessions[d.Serial]
	seedSnapshotObservations(s)
	s.observations.observe(s.device, &packets.TileState64{Rect: packets.TileBufferRect{Width: 8}})
	p := &powerTestSender{s: s}
	s.sender = p
	return c, p
}

func TestPartPowerControllerOrderingAndEffectiveState(t *testing.T) {
	d := powerDevice(40, 0, true)
	c, sender := powerController(d)
	err := c.SetLightPartPower(context.Background(), d.Serial, device.LightPartMain, false, LightPartPowerOptions{Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sender.controls, []uint16{21, 715}) || !sender.offConfirmed || sender.stagedWhileOn {
		t.Fatal("dormant preparation preceded confirmed off", sender.controls)
	}
	got := sender.s.deviceSnapshot()
	if got.PoweredOn || got.MatrixProperties.ChainZones[0][2] != d.MatrixProperties.ChainZones[0][2] || got.MatrixProperties.ChainZones[0][63].Brightness != device.ConvertExternalToDeviceValue(40, 100) {
		t.Fatal(got)
	}
	state, err := c.GetLightPartState(d.Serial, device.LightPartUplight)
	if err != nil || !state.Known || state.On || state.DevicePoweredOn || !state.Emulated || math.Abs(state.Brightness-40) > 1e-6 {
		t.Fatal(state, err)
	}
	sender.controls = nil
	err = c.SetLightPartPower(context.Background(), d.Serial, device.LightPartUplight, true, LightPartPowerOptions{Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sender.controls, []uint16{715, 21}) {
		t.Fatal(sender.controls)
	}
	got = sender.s.deviceSnapshot()
	if !got.PoweredOn || got.MatrixProperties.ChainZones[0][2].Brightness != 0 || got.MatrixProperties.ChainZones[0][63].Brightness != device.ConvertExternalToDeviceValue(40, 100) {
		t.Fatal("only uplight should become on")
	}
}

func TestPartPowerFailuresCannotStageWhileOffUnconfirmed(t *testing.T) {
	d := powerDevice(40, 0, true)
	c, sender := powerController(d)
	sender.ignoreOff = true
	err := c.SetLightPartPower(context.Background(), d.Serial, device.LightPartMain, false, LightPartPowerOptions{Timeout: 30 * time.Millisecond})
	var partial *StateSendError
	if err == nil || !errors.As(err, &partial) || partial.Sent != 1 || !reflect.DeepEqual(sender.controls, []uint16{21}) {
		t.Fatal(err, sender.controls)
	}
	c, sender = powerController(d)
	sender.failColor = true
	err = c.SetLightPartPower(context.Background(), d.Serial, device.LightPartMain, false, LightPartPowerOptions{Timeout: time.Second})
	if !errors.As(err, &partial) || partial.Sent != 1 || partial.Total != 2 {
		t.Fatal(err)
	}
	c, sender = powerController(powerDevice(0, 0, false))
	err = c.SetLightPartPower(context.Background(), d.Serial, device.LightPartMain, true, LightPartPowerOptions{Timeout: time.Second})
	if !errors.Is(err, ErrNoInheritedBrightness) || len(sender.controls) != 0 {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.SetLightPartPower(ctx, d.Serial, device.LightPartMain, true, LightPartPowerOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	bad := math.NaN()
	if err := c.SetLightPartPower(context.Background(), d.Serial, device.LightPartMain, true, LightPartPowerOptions{FallbackBrightness: &bad}); !errors.Is(err, ErrInvalidState) {
		t.Fatal(err)
	}
}

func TestPartGateWaitHonorsCancellation(t *testing.T) {
	d := powerDevice(40, 50, true)
	c, sender := powerController(d)
	release, err := sender.s.lockPartOperation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := c.SetLightPartPower(ctx, d.Serial, device.LightPartMain, false, LightPartPowerOptions{}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	if len(sender.controls) != 0 {
		t.Fatal("waiting operation sent controls")
	}
}

func TestNativePowerAndUnknownCachedPartState(t *testing.T) {
	d := device.Device{Serial: snapshotSerial(1), Address: snapshotAddr(1), ProductID: 27, Type: device.DeviceTypeLight, LightType: device.LightTypeSingleZone}
	mock := newMockClient()
	c := newSnapshotController(mock, d)
	if err := c.SetLightPartPower(context.Background(), d.Serial, device.LightPartMain, true, LightPartPowerOptions{}); err != nil {
		t.Fatal(err)
	}
	if msg := <-mock.sends; msg.Type() != 21 {
		t.Fatal("native power added colour writes")
	}
	state, err := c.GetLightPartState(d.Serial, device.LightPartMain)
	if err != nil || state.Known || state.Emulated {
		t.Fatal(state, err)
	}
	if err := c.SetLightPartPower(context.Background(), d.Serial, device.LightPartUplight, false, LightPartPowerOptions{}); !errors.Is(err, ErrUnsupportedCapability) {
		t.Fatal(err)
	}
}
