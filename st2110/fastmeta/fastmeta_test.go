package fastmeta

import (
	"bytes"
	"reflect"
	"testing"
)

func TestMarshalPayloadByteExact(t *testing.T) {
	// One Data Item: Type 0x2000A1, K=1, two content words.
	items := []DataItem{
		{Type: 0x2000A1, K: true, Contents: []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x01, 0x02, 0x03, 0x04}},
	}
	got, err := MarshalPayload(items)
	if err != nil {
		t.Fatal(err)
	}
	// header = Type<<10 | K<<9 | len(=2) = 0x2000A1<<10 | 0x200 | 0x002
	// 0x2000A1<<10 = 0x80028400 ; |0x200|0x2 = 0x80028602
	want := []byte{0x80, 0x02, 0x86, 0x02, 0xDE, 0xAD, 0xBE, 0xEF, 0x01, 0x02, 0x03, 0x04}
	if !bytes.Equal(got, want) {
		t.Fatalf("Marshal = % x\nwant      % x", got, want)
	}
}

func TestPayloadRoundTrip(t *testing.T) {
	items := []DataItem{
		{Type: 0x100, Contents: []byte{1, 2, 3, 4}},
		{Type: 0x1013FC, K: true, Contents: []byte{9, 9, 9, 9, 8, 8, 8, 8}},
		{Type: 0x3FFF00, Contents: bytes.Repeat([]byte{0x55}, 4*511)}, // max length
	}
	payload, err := MarshalPayload(items)
	if err != nil {
		t.Fatal(err)
	}
	got, err := UnmarshalPayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, items) {
		t.Fatalf("round-trip mismatch")
	}
}

func TestEmptyPayloadIsValid(t *testing.T) {
	p, err := MarshalPayload(nil)
	if err != nil || len(p) != 0 {
		t.Fatalf("empty payload: %v len=%d", err, len(p))
	}
	items, err := UnmarshalPayload(p)
	if err != nil || len(items) != 0 {
		t.Fatalf("parse empty: %v items=%d", err, len(items))
	}
}

func TestMarshalErrors(t *testing.T) {
	if _, err := MarshalPayload([]DataItem{{Type: 1, Contents: []byte{1, 2, 3}}}); err != ErrContentsAlign {
		t.Errorf("misaligned: got %v", err)
	}
	if _, err := MarshalPayload([]DataItem{{Type: 1, Contents: nil}}); err != ErrContentsRange {
		t.Errorf("empty contents: got %v", err)
	}
	if _, err := MarshalPayload([]DataItem{{Type: 0x400000, Contents: []byte{1, 2, 3, 4}}}); err != ErrTypeRange {
		t.Errorf("type range: got %v", err)
	}
}

func TestNCM(t *testing.T) {
	// CMAX = MAX(4, INT(RNOMINAL/43200)); below the knee it is 4.
	if CMax(100000) != 4 {
		t.Errorf("CMax low = %d, want 4", CMax(100000))
	}
	if CMax(432000) != 10 {
		t.Errorf("CMax = %d, want 10", CMax(432000))
	}
	// TDRAIN clamps to 1/800 at low rates.
	if got := TDrain(100); got != 1.0/800 {
		t.Errorf("TDrain low = %v, want %v", got, 1.0/800)
	}
}

func TestSDP(t *testing.T) {
	f := Format{ClockRate: 90000, SSN: "ST2110-41:2024", DIT: []uint32{0x100, 0x2000A1, 0x1013FC, 0x3FFF00}}
	m := f.MediaDescription(50000, 117)
	if m.Media != "application" {
		t.Errorf("media = %q", m.Media)
	}
	if v, _ := m.GetAttribute("rtpmap"); v != "117 ST2110-41/90000" {
		t.Errorf("rtpmap = %q", v)
	}
	// §6 example fmtp value.
	want := "117 SSN=ST2110-41:2024; DIT=100,2000A1,1013FC,3FFF00"
	if v, _ := m.GetAttribute("fmtp"); v != want {
		t.Errorf("fmtp = %q\nwant %q", v, want)
	}
	parsed, err := ParseMediaDescription(m)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(parsed, f) {
		t.Errorf("round-trip = %+v, want %+v", parsed, f)
	}
}
