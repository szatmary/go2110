package anc

import "testing"

// TestN1_MarkerBit pins RFC 8331 §2 / ST 2110-40 §5.5: the packetize path can set
// the RTP marker bit on the last ANC data packet of a field/frame.
func TestN1_MarkerBit(t *testing.T) {
	pkt := Packet{DID: 0x61, SDID: 0x02, UserData: []uint16{0x100, 0x200}}
	h := PayloadHeader{ExtendedSequenceNumber: 0xABCD, F: FFirstField}

	last, err := Packetize(h, []Packet{pkt}, PacketizeOptions{PayloadType: 100, SequenceNumber: 7, Marker: true})
	if err != nil {
		t.Fatal(err)
	}
	if !last.Header.Marker {
		t.Error("Marker should be set on the last ANC packet of a field/frame")
	}
	notLast, err := Packetize(h, []Packet{pkt}, PacketizeOptions{PayloadType: 100, SequenceNumber: 6, Marker: false})
	if err != nil {
		t.Fatal(err)
	}
	if notLast.Header.Marker {
		t.Error("Marker must not be set on a non-final ANC packet")
	}
	// Round-trips through Unmarshal.
	if _, pkts, err := Unmarshal(last.Payload); err != nil || len(pkts) != 1 {
		t.Fatalf("round-trip: %d pkts err %v", len(pkts), err)
	}
}

// TestN1_KeepAlive pins the normative keep-alive of RFC 8331 §2.1: ANC_Count 0
// (and Length 0) with the marker bit set.
func TestN1_KeepAlive(t *testing.T) {
	ka, err := KeepAlive(PayloadHeader{ExtendedSequenceNumber: 1, F: FProgressive}, PacketizeOptions{PayloadType: 100, SequenceNumber: 9})
	if err != nil {
		t.Fatal(err)
	}
	if !ka.Header.Marker {
		t.Error("keep-alive must set the marker bit")
	}
	if len(ka.Payload) != payloadHeaderLen {
		t.Fatalf("keep-alive payload = %d octets, want %d (header only)", len(ka.Payload), payloadHeaderLen)
	}
	if ka.Payload[4] != 0 { // ANC_Count
		t.Errorf("ANC_Count = %d, want 0", ka.Payload[4])
	}
	if ka.Payload[2] != 0 || ka.Payload[3] != 0 { // Length
		t.Errorf("Length = % x, want 00 00", ka.Payload[2:4])
	}
	h, pkts, err := Unmarshal(ka.Payload)
	if err != nil || len(pkts) != 0 {
		t.Fatalf("keep-alive Unmarshal: %d pkts err %v", len(pkts), err)
	}
	if h.ExtendedSequenceNumber != 1 {
		t.Errorf("ext seq = %d", h.ExtendedSequenceNumber)
	}
}

// TestN2_RejectF01 pins RFC 8331 §2.1: an F field value of 0b01 is not valid.
func TestN2_RejectF01(t *testing.T) {
	// Marshal must refuse to emit F=0b01.
	if _, err := Marshal(PayloadHeader{F: 0b01}, nil); err != ErrInvalidField {
		t.Fatalf("Marshal F=0b01 = %v, want ErrInvalidField", err)
	}
	// Unmarshal must reject a payload whose F field is 0b01.
	good, err := Marshal(PayloadHeader{F: FFirstField}, []Packet{{DID: 0x41, SDID: 0x01, UserData: []uint16{0x55}}})
	if err != nil {
		t.Fatal(err)
	}
	good[5] = 0b01 << 6 // overwrite F = 0b01
	if _, _, err := Unmarshal(good); err != ErrInvalidField {
		t.Fatalf("Unmarshal F=0b01 = %v, want ErrInvalidField", err)
	}
}

// TestN2_VerifyChecksum pins RFC 8331 §2.1/§6.2: the Checksum_Word is verified on
// parse (the buggy Unmarshal discarded it).
func TestN2_VerifyChecksum(t *testing.T) {
	pkt := Packet{DID: 0x61, SDID: 0x02, UserData: []uint16{0x040, 0x110, 0x004}}
	payload, err := Marshal(PayloadHeader{}, []Packet{pkt})
	if err != nil {
		t.Fatal(err)
	}
	// A valid payload parses cleanly.
	if _, _, err := Unmarshal(payload); err != nil {
		t.Fatalf("valid payload should parse: %v", err)
	}
	// Corrupt a low bit of the DID word (payload[12] is the first 10-bit-word
	// octet; bit 0 lies within the checksum-relevant b8..b0). The carried
	// Checksum_Word no longer matches.
	corrupt := append([]byte(nil), payload...)
	corrupt[12] ^= 0x01
	if _, _, err := Unmarshal(corrupt); err != ErrChecksum {
		t.Fatalf("corrupted payload = %v, want ErrChecksum", err)
	}
}
