package aes3

import (
	"bytes"
	"reflect"
	"testing"
	"time"
)

func TestSubframeMarshal(t *testing.T) {
	tests := []struct {
		s    Subframe
		want [4]byte
	}{
		{Subframe{BlockStart: true, FrameStart: true, Data: 0x123456}, [4]byte{0x30, 0x12, 0x34, 0x56}},
		{Subframe{BlockStart: true, FrameStart: true, P: true, C: true, U: true, V: true, Data: 0xFFFFFF}, [4]byte{0x3F, 0xFF, 0xFF, 0xFF}},
		{Subframe{FrameStart: true, Data: 0x000001}, [4]byte{0x10, 0x00, 0x00, 0x01}},
		{Subframe{V: true, Data: 0xABCDEF}, [4]byte{0x01, 0xAB, 0xCD, 0xEF}},
	}
	for _, tc := range tests {
		got := tc.s.Marshal()
		if got != tc.want {
			t.Errorf("Marshal(%+v) = % x, want % x", tc.s, got, tc.want)
		}
		if back := ParseSubframe(got[:]); back != tc.s {
			t.Errorf("ParseSubframe round-trip = %+v, want %+v", back, tc.s)
		}
	}
}

func TestPeriodsPerPacket(t *testing.T) {
	tests := []struct {
		rate int
		ms   float64
		want int
	}{
		{48000, 1, 48},
		{48000, 0.12, 6},
		{48000, 0.08, 4},
		{96000, 1, 96},
		{96000, 0.08, 8},
		{44100, 1.09, 48}, // approximate packet time rounds to 48 periods
		{44100, 0.09, 4},
	}
	for _, tc := range tests {
		f := Format{SampleRate: tc.rate, Channels: 2, PacketTime: time.Duration(tc.ms * float64(time.Millisecond))}
		if got := f.PeriodsPerPacket(); got != tc.want {
			t.Errorf("%d@%gms: periods = %d, want %d", tc.rate, tc.ms, got, tc.want)
		}
	}
}

func TestPacketizeRoundTrip(t *testing.T) {
	f := Format{SampleRate: 48000, Channels: 2, PacketTime: time.Millisecond}
	// 2 packets * 48 periods * 2 channels = 192 subframes.
	n := 2 * 48 * 2
	subs := make([]Subframe, n)
	for i := range subs {
		subs[i] = Subframe{
			BlockStart: i == 0,
			FrameStart: i%2 == 0,
			V:          i%3 == 0,
			Data:       uint32(i*0x010101) & 0xFFFFFF,
		}
	}
	pkts, err := f.Packetize(subs, PacketizeOptions{PayloadType: 97, StartTimestamp: 100, StartSequence: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(pkts) != 2 {
		t.Fatalf("got %d packets, want 2", len(pkts))
	}
	if pkts[1].Header.Timestamp != 148 { // 100 + 48 periods
		t.Errorf("packet 1 ts = %d, want 148", pkts[1].Header.Timestamp)
	}
	if len(pkts[0].Payload) != f.PacketPayloadSize() {
		t.Errorf("payload size = %d, want %d", len(pkts[0].Payload), f.PacketPayloadSize())
	}
	out, err := f.Depacketize(pkts)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out, subs) {
		t.Error("round-trip mismatch")
	}
}

func TestSDP(t *testing.T) {
	f := Format{SampleRate: 48000, Channels: 4, PacketTime: time.Millisecond}
	m := f.MediaDescription(50000, 97)
	if v, _ := m.GetAttribute("rtpmap"); v != "97 AM824/48000/4" {
		t.Errorf("rtpmap = %q", v)
	}
	if v, _ := m.GetAttribute("ptime"); v != "1" {
		t.Errorf("ptime = %q", v)
	}
	parsed, err := ParseMediaDescription(m)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.SampleRate != 48000 || parsed.Channels != 4 || parsed.PacketTime != time.Millisecond {
		t.Errorf("parsed = %+v", parsed)
	}
}

func TestValidate(t *testing.T) {
	if err := (Format{SampleRate: 48000, Channels: 3}).Validate(); err != ErrBadChannels {
		t.Errorf("odd channels: got %v", err)
	}
	if err := (Format{SampleRate: 32000, Channels: 2}).Validate(); err != ErrBadRate {
		t.Errorf("bad rate: got %v", err)
	}
	if err := PackByteAlign(); err != nil {
		t.Errorf("unexpected: %v", err)
	}
}

// PackByteAlign documents that PackSubframes always yields 4-octet alignment.
func PackByteAlign() error {
	b := PackSubframes([]Subframe{{Data: 1}, {Data: 2}})
	if len(b) != 8 || !bytes.Equal(b[4:], []byte{0, 0, 0, 2}) {
		return ErrBadPacket
	}
	return nil
}
