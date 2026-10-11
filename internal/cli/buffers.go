package cli

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/client"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/enums"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

type bufferReader func(context.Context, device.Device, []int, time.Duration) (*matrixBufferReport, error)

type matrixBufferReport struct {
	Serial        string
	Label         string
	Width, Height int
	Buffers       []matrixBufferState
}

type matrixBufferState struct {
	Index                            int
	Known                            bool
	ReceivedPackets, ExpectedPackets int
	Error                            string `json:",omitempty"`
	ColorsSHA256                     string `json:",omitempty"`
	Chains                           [][]packets.LightHsbk
}

type bufferClient interface {
	Send(*net.UDPAddr, *protocol.Message) error
	Receive(time.Duration, bool, client.HandlerFunc) error
	Close() error
}

type bufferDiscoveryClient interface {
	bufferClient
	SendBroadcast(*protocol.Message) error
}

func validateBufferIndexes(values []int) ([]int, error) {
	var result []int
	seen := make(map[int]bool)
	for _, v := range values {
		if v < 0 || v > 2 {
			return nil, fmt.Errorf("--buffer must be 0, 1 or 2")
		}
		if !seen[v] {
			result = append(result, v)
			seen[v] = true
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("at least one --buffer is required")
	}
	return result, nil
}

func validateBufferGeometry(d device.Device) error {
	if !hasLight(d) || d.LightType != device.LightTypeMatrix {
		return fmt.Errorf("buffer target must be a matrix light")
	}
	if d.Address == nil || len(d.Address.IP) == 0 || d.Address.Port <= 0 {
		return fmt.Errorf("buffer target has no usable UDP address")
	}
	m := d.MatrixProperties
	if m.Width <= 0 || m.Width > 64 || m.Height <= 0 || m.Height > 255 || m.ChainLength <= 0 || m.ChainLength > 16 || m.Width*m.Height*m.ChainLength > 65_536 {
		return fmt.Errorf("matrix geometry is missing or excessive; try a longer --discover-for")
	}
	return nil
}

func readMatrixBuffers(ctx context.Context, d device.Device, buffers []int, timeout time.Duration) (*matrixBufferReport, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateBufferGeometry(d); err != nil {
		return nil, err
	}
	var seed [4]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return nil, err
	}
	source := binary.LittleEndian.Uint32(seed[:]) | 2
	c, err := client.NewClient(&client.Config{Source: source})
	if err != nil {
		return nil, err
	}
	defer c.Close()
	// Match the controller's discovery-first socket lifecycle before querying a
	// peer directly. In particular, do not assume a second socket can immediately
	// reach a peer discovered on the first socket.
	if err := discoverBufferPeer(ctx, c, d, source, timeout); err != nil {
		return nil, fmt.Errorf("buffer query discovery: %w", err)
	}
	return readBuffersWithClient(ctx, d, buffers, timeout, c, source)
}

func discoverBufferPeer(ctx context.Context, c bufferDiscoveryClient, d device.Device, source uint32, timeout time.Duration) error {
	stop := context.AfterFunc(ctx, func() { c.Close() })
	defer stop()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := c.SendBroadcast(protocol.NewMessage(&packets.DeviceGetService{})); err != nil {
			return err
		}
		retryAt := minTime(deadline, time.Now().Add(500*time.Millisecond))
		for time.Now().Before(retryAt) {
			remaining := time.Until(retryAt)
			if remaining <= 0 {
				break
			}
			found := false
			err := c.Receive(remaining, true, func(reply *protocol.Message, from *net.UDPAddr) {
				p, service := reply.Payload.(*packets.DeviceStateService)
				found = service && p.Service == enums.DeviceServiceDEVICESERVICEUDP && int(p.Port) == d.Address.Port && from != nil && from.IP.Equal(d.Address.IP) && reply.Target() == d.Serial && reply.Source() == source
			})
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil {
				return err
			}
			if found {
				return nil
			}
		}
	}
	return fmt.Errorf("timed out discovering target on the buffer query socket")
}

func readBuffersWithClient(ctx context.Context, d device.Device, buffers []int, timeout time.Duration, c bufferClient, source uint32) (*matrixBufferReport, error) {
	if err := validateBufferGeometry(d); err != nil {
		return nil, err
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("buffer timeout must be positive")
	}
	indexes, err := validateBufferIndexes(buffers)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Closing this dedicated read socket interrupts a blocked receive on cancellation.
	stop := context.AfterFunc(ctx, func() { c.Close() })
	defer stop()
	m := d.MatrixProperties
	rows := 64 / m.Width
	expected := ((m.Height + rows - 1) / rows) * m.ChainLength
	report := &matrixBufferReport{Serial: d.Serial.String(), Label: d.Label, Width: m.Width, Height: m.Height}
	var failures []error
	var sequence uint8
	for _, fb := range indexes {
		state := matrixBufferState{Index: fb, ExpectedPackets: expected}
		chains := make([][]packets.LightHsbk, m.ChainLength)
		deadline := time.Now().Add(timeout)
		var readErr error
		for chain := 0; chain < m.ChainLength && readErr == nil; chain++ {
			chains[chain] = make([]packets.LightHsbk, m.Width*m.Height)
			for y := 0; y < m.Height; y += rows {
				sequence++
				msg := protocol.NewMessage(&packets.TileGet64{TileIndex: uint8(chain), Length: 1, Rect: packets.TileBufferRect{FbIndex: uint8(fb), Width: uint8(m.Width), Y: uint8(y)}})
				msg.SetTarget(d.Serial)
				msg.SetSource(source)
				msg.SetSequence(sequence)
				var colors [64]packets.LightHsbk
				colors, readErr = receiveBufferPacket(ctx, c, d, msg, deadline)
				if readErr != nil {
					break
				}
				copy(chains[chain][y*m.Width:], colors[:])
				state.ReceivedPackets++
			}
		}
		if readErr != nil {
			state.Error = readErr.Error()
			failures = append(failures, fmt.Errorf("buffer %d: %w", fb, readErr))
		} else {
			state.Known = true
			state.Chains = chains
			hash := sha256.New()
			for _, colors := range chains {
				for _, color := range colors {
					var b [8]byte
					binary.LittleEndian.PutUint16(b[0:2], color.Hue)
					binary.LittleEndian.PutUint16(b[2:4], color.Saturation)
					binary.LittleEndian.PutUint16(b[4:6], color.Brightness)
					binary.LittleEndian.PutUint16(b[6:8], color.Kelvin)
					hash.Write(b[:])
				}
			}
			state.ColorsSHA256 = fmt.Sprintf("%x", hash.Sum(nil))
		}
		report.Buffers = append(report.Buffers, state)
		if ctx.Err() != nil {
			break
		}
	}
	return report, errors.Join(failures...)
}

func receiveBufferPacket(ctx context.Context, c bufferClient, d device.Device, msg *protocol.Message, deadline time.Time) ([64]packets.LightHsbk, error) {
	var colors [64]packets.LightHsbk
	request := msg.Payload.(*packets.TileGet64)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return colors, err
		}
		if err := c.Send(d.Address, msg); err != nil {
			return colors, err
		}
		retryAt := minTime(deadline, time.Now().Add(500*time.Millisecond))
		for time.Now().Before(retryAt) {
			remaining := time.Until(retryAt)
			if remaining <= 0 {
				break
			}
			var received bool
			var matchErr error
			err := c.Receive(remaining, true, func(reply *protocol.Message, from *net.UDPAddr) {
				if from == nil || !from.IP.Equal(d.Address.IP) || from.Port != d.Address.Port || reply.Source() != msg.Source() || reply.Sequence() != msg.Sequence() || reply.Target() != d.Serial {
					return
				}
				if p, ok := reply.Payload.(*packets.DeviceStateUnhandled); ok && p.UnhandledType == msg.Type() {
					matchErr = fmt.Errorf("device reports TileGet64 unhandled for buffer %d", request.Rect.FbIndex)
					return
				}
				p, ok := reply.Payload.(*packets.TileState64)
				if !ok {
					return
				}
				if p.TileIndex != request.TileIndex || p.Rect != request.Rect {
					matchErr = fmt.Errorf("mismatched reply: tile=%d rectangle=%+v (requested tile=%d rectangle=%+v)", p.TileIndex, p.Rect, request.TileIndex, request.Rect)
					return
				}
				colors, received = p.Colors, true
			})
			if ctx.Err() != nil {
				return colors, ctx.Err()
			}
			if err != nil {
				return colors, err
			}
			if matchErr != nil {
				return colors, matchErr
			}
			if received {
				return colors, nil
			}
		}
	}
	return colors, fmt.Errorf("timed out waiting for matching TileState64")
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func printMatrixBuffers(out io.Writer, report *matrixBufferReport) error {
	if _, err := fmt.Fprintf(out, "%s (%s), physical frame=%dx%d; read-only buffer observations\n", safeText(report.Label), report.Serial, report.Width, report.Height); err != nil {
		return err
	}
	for _, b := range report.Buffers {
		if !b.Known {
			if _, err := fmt.Fprintf(out, "Buffer %d: unknown, received=%d/%d; %s\n", b.Index, b.ReceivedPackets, b.ExpectedPackets, safeText(b.Error)); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(out, "Buffer %d: received=%d/%d, sha256=%s\n", b.Index, b.ReceivedPackets, b.ExpectedPackets, b.ColorsSHA256); err != nil {
			return err
		}
		for chain, colors := range b.Chains {
			nonblack := 0
			for _, c := range colors {
				if c.Brightness > 0 {
					nonblack++
				}
			}
			last := len(colors) - 1
			if _, err := fmt.Fprintf(out, "  Chain %d: cells=%d, nonblack=%d, physical[0]=%s, physical[%d]=%s\n", chain, len(colors), nonblack, eventColor(device.NewColor(colors[0])), last, eventColor(device.NewColor(colors[last]))); err != nil {
				return err
			}
		}
	}
	return nil
}
