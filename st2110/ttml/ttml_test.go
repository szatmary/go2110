package ttml

import (
	"bytes"
	"strings"
	"testing"
)

func TestMarshalPacket(t *testing.T) {
	udw := []byte("<tt></tt>") // 9 bytes
	p := MarshalPacket(udw)
	want := append([]byte{0x00, 0x00, 0x00, 0x09}, udw...)
	if !bytes.Equal(p, want) {
		t.Fatalf("MarshalPacket = % x, want % x", p, want)
	}
	got, err := ParsePacket(p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, udw) {
		t.Fatalf("ParsePacket = %q", got)
	}
}

func TestPacketizeSingle(t *testing.T) {
	doc := []byte("<tt xmlns='http://www.w3.org/ns/ttml'><body/></tt>")
	pkts := Packetize(doc, PacketizeOptions{PayloadType: 112, Timestamp: 90000})
	if len(pkts) != 1 {
		t.Fatalf("got %d packets, want 1", len(pkts))
	}
	if !pkts[0].Header.Marker {
		t.Error("single packet must have marker set")
	}
	out, err := Depacketize(pkts)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, doc) {
		t.Fatalf("round-trip = %q", out)
	}
}

func TestPacketizeFragmented(t *testing.T) {
	doc := []byte(strings.Repeat("A", 1000))
	pkts := Packetize(doc, PacketizeOptions{PayloadType: 112, Timestamp: 5, StartSequence: 100, MaxUDW: 300})
	if len(pkts) != 4 { // 300+300+300+100
		t.Fatalf("got %d packets, want 4", len(pkts))
	}
	for i, p := range pkts {
		if p.Header.Timestamp != 5 {
			t.Errorf("packet %d ts = %d, want 5 (shared)", i, p.Header.Timestamp)
		}
		if p.Header.SequenceNumber != uint16(100+i) {
			t.Errorf("packet %d seq = %d", i, p.Header.SequenceNumber)
		}
		wantMarker := i == len(pkts)-1
		if p.Header.Marker != wantMarker {
			t.Errorf("packet %d marker = %v, want %v", i, p.Header.Marker, wantMarker)
		}
	}
	out, err := Depacketize(pkts)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, doc) {
		t.Errorf("fragmented round-trip mismatch (len %d)", len(out))
	}
}

func TestPacketizeUTF8Boundary(t *testing.T) {
	// 'é' is 2 bytes (0xC3 0xA9). With a budget that would split it, the cut
	// must back up to a character boundary.
	doc := []byte(strings.Repeat("é", 100)) // 200 bytes
	pkts := Packetize(doc, PacketizeOptions{PayloadType: 112, MaxUDW: 5})
	out, err := Depacketize(pkts)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, doc) {
		t.Fatal("UTF-8 fragmented round-trip mismatch")
	}
	// No fragment may end mid-rune: each fragment's bytes must be valid UTF-8
	// prefixes when concatenated; check each UDW length is even (2-byte runes).
	for i, p := range pkts {
		udw, _ := ParsePacket(p.Payload)
		if len(udw)%2 != 0 {
			t.Errorf("fragment %d splits a 2-byte rune (len %d)", i, len(udw))
		}
	}
}

func TestKeepAlive(t *testing.T) {
	ka := KeepAlive(PacketizeOptions{PayloadType: 112, Timestamp: 1})
	if !ka.Header.Marker {
		t.Error("keep-alive must set marker")
	}
	udw, err := ParsePacket(ka.Payload)
	if err != nil || len(udw) != 0 {
		t.Errorf("keep-alive UDW len = %d err=%v", len(udw), err)
	}
}

func TestSDP(t *testing.T) {
	m := MediaDescription(30000, 112, "")
	if m.Media != "application" {
		t.Errorf("media = %q", m.Media)
	}
	if v, _ := m.GetAttribute("rtpmap"); v != "112 ttml+xml/90000" {
		t.Errorf("rtpmap = %q", v)
	}
}
