package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/client"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/enums"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

type fakeBufferClient struct {
	d                                     device.Device
	request                               *protocol.Message
	requests                              []*packets.TileGet64
	broadcasts                            int
	wrongBuffer, unrelated, missingSecond bool
	unhandled                             bool
	onSend                                func()
}

func (c *fakeBufferClient) Close() error { return nil }
func (c *fakeBufferClient) SendBroadcast(msg *protocol.Message) error {
	if _, ok := msg.Payload.(*packets.DeviceGetService); !ok {
		return errors.New("unexpected broadcast")
	}
	msg.SetSource(42)
	c.request = msg
	c.broadcasts++
	return nil
}
func (c *fakeBufferClient) Send(_ *net.UDPAddr, msg *protocol.Message) error {
	p, ok := msg.Payload.(*packets.TileGet64)
	if !ok {
		return errors.New("unexpected control packet")
	}
	copy := *p
	c.requests = append(c.requests, &copy)
	c.request = msg
	if c.onSend != nil {
		c.onSend()
	}
	return nil
}
func (c *fakeBufferClient) Receive(timeout time.Duration, _ bool, callback client.HandlerFunc) error {
	if _, ok := c.request.Payload.(*packets.DeviceGetService); ok {
		reply := protocol.NewMessage(&packets.DeviceStateService{Service: enums.DeviceServiceDEVICESERVICEUDP, Port: uint32(c.d.Address.Port)})
		reply.SetTarget(c.d.Serial)
		reply.SetSource(c.request.Source())
		if c.unrelated {
			reply.SetSource(reply.Source() + 1)
			c.unrelated = false
		}
		callback(reply, c.d.Address)
		return nil
	}
	req := c.request.Payload.(*packets.TileGet64)
	if c.missingSecond && req.Rect.Y != 0 {
		time.Sleep(timeout)
		return nil
	}
	reply := protocol.NewMessage(&packets.TileState64{TileIndex: req.TileIndex, Rect: req.Rect})
	reply.SetTarget(c.d.Serial)
	reply.SetSource(c.request.Source())
	reply.SetSequence(c.request.Sequence())
	if c.unhandled {
		reply.Payload = &packets.DeviceStateUnhandled{UnhandledType: c.request.Type()}
		callback(reply, c.d.Address)
		return nil
	}
	p := reply.Payload.(*packets.TileState64)
	for i := range p.Colors {
		p.Colors[i] = packets.LightHsbk{Hue: uint16(int(req.Rect.Y)*int(req.Rect.Width) + i), Brightness: uint16(req.Rect.FbIndex) * 100, Kelvin: 3500}
	}
	if c.wrongBuffer {
		p.Rect.FbIndex = 0
	}
	if c.unrelated {
		reply.SetSource(reply.Source() + 1)
		callback(reply, c.d.Address)
		c.unrelated = false
		return nil
	}
	callback(reply, c.d.Address)
	return nil
}

func bufferMatrix() device.Device {
	d := themeLight()
	d.LightType = device.LightTypeMatrix
	d.Address = &net.UDPAddr{IP: net.ParseIP("192.168.1.176"), Port: 56700}
	d.MatrixProperties = device.MatrixProperties{Width: 8, Height: 8, ChainLength: 1}
	return d
}

func TestReadMatrixBuffersMatchingReceiptsAndBlack(t *testing.T) {
	d := bufferMatrix()
	c := &fakeBufferClient{d: d, unrelated: true}
	report, err := readBuffersWithClient(context.Background(), d, []int{0, 1, 2}, time.Second, c, 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Buffers) != 3 {
		t.Fatal(report)
	}
	for i, b := range report.Buffers {
		if !b.Known || b.ReceivedPackets != 1 || b.ExpectedPackets != 1 || len(b.Chains[0]) != 64 || b.Chains[0][63].Brightness != uint16(i)*100 || b.ColorsSHA256 == "" {
			t.Fatal(b)
		}
	}
	if report.Buffers[0].ColorsSHA256 == report.Buffers[1].ColorsSHA256 {
		t.Fatal("hash ignored brightness")
	}
	if d.MatrixProperties.ChainZones != nil {
		t.Fatal("read changed cached visible state")
	}
}

func TestBufferSocketDiscoveryBeforeDirectQueries(t *testing.T) {
	d := bufferMatrix()
	c := &fakeBufferClient{d: d, unrelated: true}
	if err := discoverBufferPeer(context.Background(), c, d, 42, time.Second); err != nil {
		t.Fatal(err)
	}
	if c.broadcasts != 1 || len(c.requests) != 0 {
		t.Fatal("bootstrap must only send GetService")
	}
	report, err := readBuffersWithClient(context.Background(), d, []int{0}, time.Second, c, 42)
	if err != nil || !report.Buffers[0].Known || len(c.requests) != 1 {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := discoverBufferPeer(ctx, c, d, 42, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestReadMatrixBuffersChunksChainsAndPartialFailure(t *testing.T) {
	d := bufferMatrix()
	d.MatrixProperties.Width, d.MatrixProperties.Height, d.MatrixProperties.ChainLength = 16, 8, 2
	c := &fakeBufferClient{d: d}
	report, err := readBuffersWithClient(context.Background(), d, []int{2}, time.Second, c, 42)
	if err != nil {
		t.Fatal(err)
	}
	b := report.Buffers[0]
	if !b.Known || b.ExpectedPackets != 4 || b.ReceivedPackets != 4 || len(b.Chains) != 2 || b.Chains[1][127].Hue != 127 {
		t.Fatal(b)
	}
	if len(c.requests) != 4 || c.requests[1].Rect.Y != 4 || c.requests[2].TileIndex != 1 {
		t.Fatal(c.requests)
	}
	c = &fakeBufferClient{d: d, missingSecond: true}
	report, err = readBuffersWithClient(context.Background(), d, []int{2}, 20*time.Millisecond, c, 42)
	if err == nil || report.Buffers[0].Known || report.Buffers[0].Chains != nil || report.Buffers[0].ColorsSHA256 != "" || report.Buffers[0].ReceivedPackets != 1 {
		t.Fatalf("report=%+v err=%v", report, err)
	}
}

func TestReadMatrixBuffersRejectsIgnoredBufferAndCancellation(t *testing.T) {
	d := bufferMatrix()
	c := &fakeBufferClient{d: d, wrongBuffer: true}
	report, err := readBuffersWithClient(context.Background(), d, []int{1}, time.Second, c, 42)
	if err == nil || report.Buffers[0].Known || !strings.Contains(report.Buffers[0].Error, "mismatched reply") {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	c = &fakeBufferClient{d: d, unhandled: true}
	report, err = readBuffersWithClient(context.Background(), d, []int{2}, time.Second, c, 42)
	if err == nil || report.Buffers[0].Known || !strings.Contains(report.Buffers[0].Error, "unhandled") {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	c = &fakeBufferClient{d: d, onSend: cancel}
	_, err = readBuffersWithClient(ctx, d, []int{0, 1, 2}, time.Second, c, 42)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestBufferCLISelectionFlagsAndOutput(t *testing.T) {
	for _, format := range []string{"text", "json"} {
		f := &fakeBackend{devices: []device.Device{bufferMatrix()}}
		var out bytes.Buffer
		read := func(_ context.Context, d device.Device, indexes []int, timeout time.Duration) (*matrixBufferReport, error) {
			if d.Serial != f.devices[0].Serial || !reflect.DeepEqual(indexes, []int{2}) || timeout != time.Second {
				t.Fatal(d, indexes, timeout)
			}
			return readBuffersWithClient(context.Background(), d, indexes, timeout, &fakeBufferClient{d: d}, 42)
		}
		err := runWithBufferReader(context.Background(), []string{"devices", "buffers", "--target", "Desk", "--buffer", "2", "--timeout", "1s", "--discover-for", "1ns", "--output", format}, &out, io.Discard, factoryFor(f), read)
		if err != nil || f.sends != 0 || f.captures != 0 || f.restores != 0 || !f.closed {
			t.Fatalf("err=%v fake=%+v", err, f)
		}
		if format == "text" && !strings.Contains(out.String(), "physical[63]") {
			t.Fatal(out.String())
		}
		if format == "json" && (!strings.Contains(out.String(), `"Known": true`) || !strings.Contains(out.String(), `"Chains"`)) {
			t.Fatal(out.String())
		}
	}
}

func TestBufferCLIInvalidIndexesOfflineAndFailuresVisible(t *testing.T) {
	err := run(context.Background(), []string{"devices", "buffers", "--target", "Desk", "--buffer", "3"}, io.Discard, io.Discard, func(bool, io.Writer) (backend, error) { t.Fatal("invalid index opened controller"); return nil, nil })
	if err == nil {
		t.Fatal("invalid index accepted")
	}
	f := &fakeBackend{devices: []device.Device{bufferMatrix()}}
	var out bytes.Buffer
	err = runWithBufferReader(context.Background(), []string{"devices", "buffers", "--target", "Desk", "--discover-for", "1ns"}, &out, io.Discard, factoryFor(f), func(_ context.Context, d device.Device, indexes []int, timeout time.Duration) (*matrixBufferReport, error) {
		if !reflect.DeepEqual(indexes, []int{0, 1, 2}) {
			t.Fatal(indexes)
		}
		return readBuffersWithClient(context.Background(), d, indexes, timeout, &fakeBufferClient{d: d, wrongBuffer: true}, 42)
	})
	if err == nil || !strings.Contains(out.String(), "Buffer 1: unknown") || f.sends != 0 {
		t.Fatal(out.String(), err)
	}
}
