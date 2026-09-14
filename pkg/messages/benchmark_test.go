package messages

import (
	"testing"

	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

var (
	benchmarkControlMessagesSink []*protocol.Message
	benchmarkControlBytesSink    []byte
)

func BenchmarkMultizoneColorBatch(b *testing.B) {
	colors := make([]packets.LightHsbk, 82)
	for i := range colors {
		colors[i] = packets.LightHsbk{Hue: uint16(i * 700), Saturation: 65535, Brightness: 32768, Kelvin: 3500}
	}

	sample := SetMultizoneExtendedColors(0, colors, 0)
	packetCount, wireBytes := controlBatchMetrics(b, sample)
	b.ReportAllocs()
	for b.Loop() {
		msgs := SetMultizoneExtendedColors(0, colors, 0)
		marshalControlBatch(b, msgs)
		benchmarkControlMessagesSink = msgs
	}
	b.ReportMetric(float64(packetCount), "packets/op")
	b.ReportMetric(float64(wireBytes), "wire-B/op")
}

func BenchmarkMatrixColorBatch(b *testing.B) {
	colors := make([]packets.LightHsbk, 128)
	for i := range colors {
		colors[i] = packets.LightHsbk{Hue: uint16(i * 500), Saturation: 65535, Brightness: 32768, Kelvin: 3500}
	}

	sample := SetMatrixColorsFromSlice(0, 1, 16, colors, 0)
	packetCount, wireBytes := controlBatchMetrics(b, sample)
	b.ReportAllocs()
	for b.Loop() {
		msgs := SetMatrixColorsFromSlice(0, 1, 16, colors, 0)
		marshalControlBatch(b, msgs)
		benchmarkControlMessagesSink = msgs
	}
	b.ReportMetric(float64(packetCount), "packets/op")
	b.ReportMetric(float64(wireBytes), "wire-B/op")
}

func controlBatchMetrics(b *testing.B, msgs []*protocol.Message) (int, int) {
	b.Helper()
	wireBytes := 0
	for _, msg := range msgs {
		encoded, err := msg.MarshalBinary()
		if err != nil {
			b.Fatal(err)
		}
		wireBytes += len(encoded)
	}
	return len(msgs), wireBytes
}

func marshalControlBatch(b *testing.B, msgs []*protocol.Message) {
	b.Helper()
	for _, msg := range msgs {
		encoded, err := msg.MarshalBinary()
		if err != nil {
			b.Fatal(err)
		}
		benchmarkControlBytesSink = encoded
	}
}
