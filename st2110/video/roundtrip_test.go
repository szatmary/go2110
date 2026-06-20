package video

import (
	"reflect"
	"testing"

	"github.com/szatmary/go2110/rtp"
	"github.com/szatmary/go2110/st2110/media"
)

// fillFrame populates every plane with deterministic values within the depth.
func fillFrame(fr *Frame) {
	mask := uint16((1 << fr.Format.Depth.Bits()) - 1)
	for p, plane := range fr.Planes {
		for i := range plane {
			plane[i] = uint16(p*101+i*7+1) & mask
		}
	}
}

func TestPacketizeRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		f    Format
	}{
		{"422-10 16x4", Format{Sampling: SamplingYCbCr422, Depth: Depth10, Width: 16, Height: 4}},
		{"422-8 16x4", Format{Sampling: SamplingYCbCr422, Depth: Depth8, Width: 16, Height: 4}},
		{"422-12 16x4", Format{Sampling: SamplingYCbCr422, Depth: Depth12, Width: 16, Height: 4}},
		{"422-16 16x4", Format{Sampling: SamplingYCbCr422, Depth: Depth16, Width: 16, Height: 4}},
		{"444-10 16x4", Format{Sampling: SamplingYCbCr444, Depth: Depth10, Width: 16, Height: 4}},
		{"444-10 fill 6x2", Format{Sampling: SamplingYCbCr444, Depth: Depth10, Width: 6, Height: 2}},
		{"444-8 8x2", Format{Sampling: SamplingYCbCr444, Depth: Depth8, Width: 8, Height: 2}},
		{"rgb-10 8x2", Format{Sampling: SamplingRGB, Depth: Depth10, Width: 8, Height: 2}},
		{"rgb-8 8x2", Format{Sampling: SamplingRGB, Depth: Depth8, Width: 8, Height: 2}},
		{"420-10 16x4", Format{Sampling: SamplingYCbCr420, Depth: Depth10, Width: 16, Height: 4}},
		{"420-8 16x4", Format{Sampling: SamplingYCbCr420, Depth: Depth8, Width: 16, Height: 4}},
		{"420-12 16x4", Format{Sampling: SamplingYCbCr420, Depth: Depth12, Width: 16, Height: 4}},
		{"key-10 16x2", Format{Sampling: SamplingKey, Depth: Depth10, Width: 16, Height: 2}},
		{"key-8 16x2", Format{Sampling: SamplingKey, Depth: Depth8, Width: 16, Height: 2}},
		{"422-10 large 1280x4", Format{Sampling: SamplingYCbCr422, Depth: Depth10, Width: 1280, Height: 4}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orig := NewFrame(tc.f)
			fillFrame(orig)

			pf, err := orig.Pack()
			if err != nil {
				t.Fatal(err)
			}
			pkts, err := pf.Packetize(PacketizeOptions{
				PayloadType: 96, SSRC: 0x11223344, Timestamp: 12345, StartSequence: 0xFFFE,
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(pkts) == 0 {
				t.Fatal("no packets produced")
			}
			// Exactly one marker bit, on the last packet.
			for i, p := range pkts {
				if p.Header.Marker != (i == len(pkts)-1) {
					t.Errorf("packet %d marker = %v", i, p.Header.Marker)
				}
				if p.Header.Timestamp != 12345 {
					t.Errorf("packet %d timestamp = %d", i, p.Header.Timestamp)
				}
			}
			// Sequence numbers increment from the low 16 bits of StartSequence.
			if pkts[0].Header.SequenceNumber != 0xFFFE {
				t.Errorf("first seq = %#x", pkts[0].Header.SequenceNumber)
			}

			// Round-trip through marshal/unmarshal then depacketize + unpack.
			wire := make([]rtp.Packet, len(pkts))
			for i, p := range pkts {
				b, err := p.Marshal()
				if err != nil {
					t.Fatal(err)
				}
				var rp rtp.Packet
				if err := rp.Unmarshal(b); err != nil {
					t.Fatal(err)
				}
				wire[i] = rp
			}

			got, err := Depacketize(wire, tc.f)
			if err != nil {
				t.Fatal(err)
			}
			out, err := got.Unpack()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(out.Planes, orig.Planes) {
				t.Errorf("planes differ after round-trip")
			}
		})
	}
}

func TestBlockPackingMode(t *testing.T) {
	f := Format{Sampling: SamplingYCbCr422, Depth: Depth10, Width: 1280, Height: 4, PackingMode: PackingBPM}
	orig := NewFrame(f)
	fillFrame(orig)
	pf, err := orig.Pack()
	if err != nil {
		t.Fatal(err)
	}
	pkts, err := pf.Packetize(PacketizeOptions{PayloadType: 96, Timestamp: 1})
	if err != nil {
		t.Fatal(err)
	}
	// Every packet except the last must carry exactly 7*180 = 1260 octets of
	// Sample Row Data (ST 2110-20 §6.3.3).
	for i, p := range pkts {
		var ph PayloadHeader
		n, err := ph.Unmarshal(p.Payload)
		if err != nil {
			t.Fatal(err)
		}
		dataLen := len(p.Payload) - n
		if i < len(pkts)-1 {
			if dataLen != BPMBlock*BPMBlocksPerPacket {
				t.Errorf("packet %d data len = %d, want 1260", i, dataLen)
			}
			if dataLen%BPMBlock != 0 {
				t.Errorf("packet %d data not multiple of 180", i)
			}
		}
		if len(ph.SRDs) > MaxSRD {
			t.Errorf("packet %d has %d SRDs", i, len(ph.SRDs))
		}
	}
	// Round-trip correctness.
	got, err := Depacketize(pkts, f)
	if err != nil {
		t.Fatal(err)
	}
	out, err := got.Unpack()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out.Planes, orig.Planes) {
		t.Error("BPM round-trip mismatch")
	}
}

func TestFrameTimestampWiring(t *testing.T) {
	// A small integration: timestamps come from media.Clock and stay constant
	// across a frame's packets (ST 2110-20 §6.1.3).
	c := media.VideoClock()
	ts := c.FrameTimestamp(10, media.Rational{Num: 25, Den: 1})
	if ts != 10*3600 {
		t.Fatalf("ts = %d", ts)
	}
}

func TestInterlacedRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		f    Format
	}{
		{"422-10 interlaced even", Format{Sampling: SamplingYCbCr422, Depth: Depth10, Width: 16, Height: 8, Interlaced: true}},
		{"422-10 interlaced odd", Format{Sampling: SamplingYCbCr422, Depth: Depth10, Width: 16, Height: 5, Interlaced: true}},
		{"444-10 PsF", Format{Sampling: SamplingYCbCr444, Depth: Depth10, Width: 8, Height: 6, Interlaced: true, Segmented: true}},
		{"rgb-8 interlaced", Format{Sampling: SamplingRGB, Depth: Depth8, Width: 8, Height: 4, Interlaced: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orig := NewFrame(tc.f)
			fillFrame(orig)
			pf, err := orig.Pack()
			if err != nil {
				t.Fatal(err)
			}
			// Field row numbers restart at 0 per field; the F bit distinguishes them.
			var sawField0, sawField1 bool
			for _, ln := range pf.Lines {
				if ln.Field {
					sawField1 = true
				} else {
					sawField0 = true
				}
			}
			if !sawField0 || !sawField1 {
				t.Fatalf("expected both fields, field0=%v field1=%v", sawField0, sawField1)
			}

			c := media.VideoClock()
			fps := media.Rational{Num: 30000, Den: 1001}
			ts0 := c.FieldTimestamp(0, 0, fps)
			ts1 := c.FieldTimestamp(0, 1, fps)
			if tc.f.Segmented {
				ts1 = ts0 // PsF: both segments share the timestamp (§7.6.1)
			}
			pkts, err := pf.PacketizeFields(PacketizeOptions{PayloadType: 96, SSRC: 1, StartSequence: 0}, ts0, ts1)
			if err != nil {
				t.Fatal(err)
			}
			// Exactly two marker bits (one per field), and the field timestamps used.
			markers := 0
			for _, p := range pkts {
				if p.Header.Marker {
					markers++
				}
				if p.Header.Timestamp != ts0 && p.Header.Timestamp != ts1 {
					t.Errorf("unexpected timestamp %d", p.Header.Timestamp)
				}
			}
			if markers != 2 {
				t.Errorf("got %d marker bits, want 2 (one per field)", markers)
			}

			got, err := Depacketize(pkts, tc.f)
			if err != nil {
				t.Fatal(err)
			}
			out, err := got.Unpack()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(out.Planes, orig.Planes) {
				t.Error("interlaced planes differ after round-trip")
			}
		})
	}
}
