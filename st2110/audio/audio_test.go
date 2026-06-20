package audio

import (
	"bytes"
	"reflect"
	"testing"
)

func TestSamplesPerPacket(t *testing.T) {
	tests := []struct {
		rate int
		pt   string
		want int
	}{
		{48000, "1ms", 48},
		{48000, "125us", 6},
		{96000, "1ms", 96},
		{96000, "125us", 12},
		{44100, "1ms", 0}, // 44.1k @ 1ms is not an integer; reported via Validate
	}
	for _, tc := range tests {
		var pt = PacketTime1ms
		if tc.pt == "125us" {
			pt = PacketTime125us
		}
		f := Format{Encoding: L24, SampleRate: tc.rate, Channels: 2, PacketTime: pt}
		got := f.SamplesPerPacket()
		if got != tc.want {
			t.Errorf("%d@%s: SamplesPerPacket = %d, want %d", tc.rate, tc.pt, got, tc.want)
		}
	}
}

func TestPacketPayloadSize(t *testing.T) {
	f := Format{Encoding: L24, SampleRate: 48000, Channels: 2, PacketTime: PacketTime1ms}
	// 48 samples * 2 ch * 3 bytes = 288 octets.
	if got := f.PacketPayloadSize(); got != 288 {
		t.Errorf("PacketPayloadSize = %d, want 288", got)
	}
	f16 := Format{Encoding: L16, SampleRate: 48000, Channels: 8, PacketTime: PacketTime125us}
	// 6 samples * 8 ch * 2 bytes = 96 octets.
	if got := f16.PacketPayloadSize(); got != 96 {
		t.Errorf("L16 PacketPayloadSize = %d, want 96", got)
	}
}

func TestPackUnpackL24(t *testing.T) {
	f := Format{Encoding: L24, Channels: 1}
	samples := []int32{0x123456, 0x7FFFFF, -1, -8388608, 0}
	want := []byte{
		0x12, 0x34, 0x56,
		0x7F, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF,
		0x80, 0x00, 0x00,
		0x00, 0x00, 0x00,
	}
	got, err := f.PackSamples(samples)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("PackSamples = % x\nwant % x", got, want)
	}
	back, err := f.UnpackSamples(got)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, samples) {
		t.Fatalf("UnpackSamples = %v, want %v", back, samples)
	}
}

func TestPackUnpackL16(t *testing.T) {
	f := Format{Encoding: L16, Channels: 2}
	samples := []int32{0x1234, -1, 0x7FFF, -32768}
	want := []byte{0x12, 0x34, 0xFF, 0xFF, 0x7F, 0xFF, 0x80, 0x00}
	got, err := f.PackSamples(samples)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("PackSamples = % x, want % x", got, want)
	}
	back, _ := f.UnpackSamples(got)
	if !reflect.DeepEqual(back, samples) {
		t.Fatalf("round-trip = %v", back)
	}
}

func TestPacketizeTimestamps(t *testing.T) {
	f := Format{Encoding: L24, SampleRate: 48000, Channels: 2, PacketTime: PacketTime1ms}
	// 3 packets worth of stereo samples = 3*48*2 = 288 int32s.
	periods := 3 * 48
	samples := make([]int32, periods*2)
	for i := range samples {
		samples[i] = int32(i % 1000)
	}
	pkts, err := f.Packetize(samples, PacketizeOptions{PayloadType: 101, SSRC: 7, StartTimestamp: 1000, StartSequence: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(pkts) != 3 {
		t.Fatalf("got %d packets, want 3", len(pkts))
	}
	for i, p := range pkts {
		if p.Header.SequenceNumber != uint16(5+i) {
			t.Errorf("packet %d seq = %d", i, p.Header.SequenceNumber)
		}
		if p.Header.Timestamp != uint32(1000+i*48) {
			t.Errorf("packet %d ts = %d, want %d", i, p.Header.Timestamp, 1000+i*48)
		}
		if len(p.Payload) != f.PacketPayloadSize() {
			t.Errorf("packet %d payload = %d, want %d", i, len(p.Payload), f.PacketPayloadSize())
		}
	}
	// Round-trip.
	out, err := f.Depacketize(pkts)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out, samples) {
		t.Error("Depacketize mismatch")
	}
}

func TestValidate(t *testing.T) {
	good := Format{Encoding: L24, SampleRate: 48000, Channels: 2, PacketTime: PacketTime1ms}
	if err := good.Validate(); err != nil {
		t.Errorf("good format: %v", err)
	}
	bad := Format{Encoding: L24, SampleRate: 44100, Channels: 2, PacketTime: PacketTime1ms}
	if err := bad.Validate(); err != ErrBadPacketTime {
		t.Errorf("44.1k@1ms: got %v, want ErrBadPacketTime", err)
	}
}
