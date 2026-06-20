package video

import (
	"testing"

	"github.com/szatmary/go2110/st2110/media"
)

// TestM2_PacketizeFrameSharedClockTimestamp pins ST 2110-10 §7.6.1: every RTP
// packet of one frame shares the frame's sampling-instant timestamp, derived
// through the media clock. Previously the clock's frame-timestamp derivation had
// no caller and the packetizer took a raw, caller-supplied Timestamp.
func TestM2_PacketizeFrameSharedClockTimestamp(t *testing.T) {
	f := Format{
		Sampling:       SamplingYCbCr422,
		Depth:          Depth10,
		Width:          1920,
		Height:         8,
		ExactFrameRate: media.Rational{Num: 60000, Den: 1001},
	}
	fr := NewFrame(f)
	pf, err := fr.Pack()
	if err != nil {
		t.Fatal(err)
	}
	clock := media.VideoClock()

	const idx = 1
	// 90 kHz / (60000/1001): floor(1 × 90000 × 1001 / 60000) = floor(1501.5) = 1501.
	want := clock.FrameTimestamp(idx, f.ExactFrameRate)
	if want != 1501 {
		t.Fatalf("spec check: FrameTimestamp(1) = %d, want 1501", want)
	}

	pkts, err := pf.PacketizeFrame(clock, idx, PacketizeOptions{PayloadType: 96})
	if err != nil {
		t.Fatal(err)
	}
	if len(pkts) < 2 {
		t.Fatalf("expected multiple packets, got %d", len(pkts))
	}
	for i, p := range pkts {
		if p.Header.Timestamp != want {
			t.Errorf("packet %d timestamp = %d, want %d (all packets of a frame share the §7.6.1 timestamp)", i, p.Header.Timestamp, want)
		}
	}
}

// TestM2_PacketizeFrameFieldsTimestamps pins the §7.6.1 field-offset derivation
// through the clock for interlaced video.
func TestM2_PacketizeFrameFieldsTimestamps(t *testing.T) {
	f := Format{
		Sampling:       SamplingYCbCr422,
		Depth:          Depth10,
		Width:          1920,
		Height:         8,
		Interlaced:     true,
		ExactFrameRate: media.Rational{Num: 30, Den: 1},
	}
	fr := NewFrame(f)
	pf, err := fr.Pack()
	if err != nil {
		t.Fatal(err)
	}
	clock := media.VideoClock()
	const idx = 2
	wantFirst := clock.FieldTimestamp(idx, 0, f.ExactFrameRate)
	wantSecond := clock.FieldTimestamp(idx, 1, f.ExactFrameRate)
	// 30 fps at 90 kHz: frame ts = 2*3000 = 6000; half-field offset = floor(90000/(2*30)) = 1500.
	if wantFirst != 6000 || wantSecond != 7500 {
		t.Fatalf("spec check: first=%d second=%d, want 6000/7500", wantFirst, wantSecond)
	}
	pkts, err := pf.PacketizeFrameFields(clock, idx, PacketizeOptions{PayloadType: 96})
	if err != nil {
		t.Fatal(err)
	}
	var sawFirst, sawSecond bool
	for _, p := range pkts {
		switch p.Header.Timestamp {
		case wantFirst:
			sawFirst = true
		case wantSecond:
			sawSecond = true
		default:
			t.Errorf("packet timestamp %d is neither field timestamp (%d/%d)", p.Header.Timestamp, wantFirst, wantSecond)
		}
	}
	if !sawFirst || !sawSecond {
		t.Errorf("expected both field timestamps present (first=%v second=%v)", sawFirst, sawSecond)
	}
}
