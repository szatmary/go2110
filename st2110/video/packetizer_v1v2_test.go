package video

import (
	"testing"

	"github.com/szatmary/go2110/st2110/media"
)

// TestV1_UDPDatagramWithinLimit pins ST 2110-10 §6.3: the Standard UDP Size
// Limit (1460 octets) bounds the whole UDP datagram, i.e. the 8-octet UDP header
// + 12-octet RTP header + RTP payload. The buggy budget subtracted only the RTP
// header, so worst-case rows produced 1468-octet datagrams.
func TestV1_UDPDatagramWithinLimit(t *testing.T) {
	f := Format{
		Sampling:       SamplingYCbCr422,
		Depth:          Depth10,
		Width:          1920,
		Height:         4,
		ExactFrameRate: media.Rational{Num: 60, Den: 1},
	}
	fr := NewFrame(f)
	pf, err := fr.Pack()
	if err != nil {
		t.Fatal(err)
	}
	pkts, err := pf.Packetize(PacketizeOptions{PayloadType: 96})
	if err != nil {
		t.Fatal(err)
	}
	if len(pkts) < 2 {
		t.Fatalf("expected several packets, got %d", len(pkts))
	}
	maxDatagram := 0
	for i, p := range pkts {
		datagram := udpHeaderLen + rtpFixedHeaderLen + len(p.Payload)
		if datagram > maxDatagram {
			maxDatagram = datagram
		}
		if datagram > StandardUDPLimit {
			t.Errorf("packet %d datagram %d octets exceeds Standard UDP Size Limit %d", i, datagram, StandardUDPLimit)
		}
	}
	// Guard against a vacuous pass: at least one packet must be near the limit.
	if maxDatagram < StandardUDPLimit-rtpFixedHeaderLen {
		t.Fatalf("no near-limit packet produced (max datagram %d); test would not catch the overflow", maxDatagram)
	}
}

// TestV1_ExtendedUDPLimit checks datagram accounting against the Extended UDP
// Size Limit (8960 octets, ST 2110-10 §6.4 / Annex A).
func TestV1_ExtendedUDPLimit(t *testing.T) {
	f := Format{
		Sampling:       SamplingYCbCr422,
		Depth:          Depth10,
		Width:          3840,
		Height:         2,
		ExactFrameRate: media.Rational{Num: 60, Den: 1},
		MaxUDP:         ExtendedUDPLimit,
	}
	fr := NewFrame(f)
	pf, err := fr.Pack()
	if err != nil {
		t.Fatal(err)
	}
	pkts, err := pf.Packetize(PacketizeOptions{PayloadType: 96})
	if err != nil {
		t.Fatal(err)
	}
	for i, p := range pkts {
		if d := udpHeaderLen + rtpFixedHeaderLen + len(p.Payload); d > ExtendedUDPLimit {
			t.Errorf("packet %d datagram %d exceeds Extended UDP Size Limit %d", i, d, ExtendedUDPLimit)
		}
	}
}

// TestV2_InterlacedPacketizeErrors pins the §7.6.1 timestamp rule: Packetize
// emits a single shared RTP timestamp, which is wrong for interlaced/PsF, so it
// must hard-error rather than silently produce field-mixed packets.
func TestV2_InterlacedPacketizeErrors(t *testing.T) {
	f := Format{
		Sampling:       SamplingYCbCr422,
		Depth:          Depth10,
		Width:          16,
		Height:         4,
		Interlaced:     true,
		ExactFrameRate: media.Rational{Num: 30, Den: 1},
	}
	fr := NewFrame(f)
	pf, err := fr.Pack()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pf.Packetize(PacketizeOptions{PayloadType: 96}); err != ErrInterlacedUsePacketizeFields {
		t.Fatalf("Packetize on interlaced format = %v, want ErrInterlacedUsePacketizeFields", err)
	}
	// PacketizeFields must still work for the same frame.
	if _, err := pf.PacketizeFields(PacketizeOptions{PayloadType: 96}, 100, 200); err != nil {
		t.Fatalf("PacketizeFields: %v", err)
	}
}
