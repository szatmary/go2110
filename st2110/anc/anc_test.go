package anc

import (
	"bytes"
	"reflect"
	"testing"
)

func TestEvenParity10(t *testing.T) {
	tests := []struct {
		in   uint8
		want uint16
	}{
		{0x00, 0x200}, // 0 data bits set: b8=0, b9=1
		{0x01, 0x101}, // 1 bit: b8=1, b9=0
		{0x03, 0x203}, // 2 bits: b8=0, b9=1
		{0x07, 0x107}, // 3 bits: b8=1, b9=0
		{0xFF, 0x2FF}, // 8 bits: b8=0, b9=1
		{0x61, 0x161}, // 0b01100001, 3 bits: b8=1, b9=0
	}
	for _, tc := range tests {
		if got := EvenParity10(tc.in); got != tc.want {
			t.Errorf("EvenParity10(%#x) = %#x, want %#x", tc.in, got, tc.want)
		}
		// The low 8 bits must always recover the input value.
		if uint8(EvenParity10(tc.in)) != tc.in {
			t.Errorf("low byte of EvenParity10(%#x) != input", tc.in)
		}
	}
}

func TestChecksum10(t *testing.T) {
	// Sum of nine LSBs, with b9 = NOT b8.
	cs := Checksum10(0x200, 0x200, 0x200) // 0+0+0 = 0 -> b8=0 -> b9=1
	if cs != 0x200 {
		t.Errorf("Checksum10 zeros = %#x, want 0x200", cs)
	}
	cs = Checksum10(0x101, 0x001) // 0x101&0x1ff=0x101, +1 = 0x102 -> b8=1 -> b9=0
	if cs != 0x102 {
		t.Errorf("Checksum10 = %#x, want 0x102", cs)
	}
}

func TestMarshalByteExact(t *testing.T) {
	// One empty ANC packet on line 9: validates location header, 10-bit word
	// packing (DID/SDID/DC/CS), parity, and word_align (RFC 8331 §2.1).
	pkts := []Packet{{LineNumber: 9}}
	got, err := Marshal(PayloadHeader{}, pkts)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{
		0x00, 0x00, 0x00, 0x0C, 0x01, 0x00, 0x00, 0x00, // header: extseq 0, len 12, count 1, F 0
		0x00, 0x90, 0x00, 0x00, // C=0 line=9 hoff=0 S=0 stream=0
		0x80, 0x20, 0x08, 0x02, 0x00, // DID/SDID/DC/CS = 0x200 x4 (40 bits)
		0x00, 0x00, 0x00, // word_align to 32-bit boundary
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("Marshal =\n % x\nwant\n % x", got, want)
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	in := []Packet{
		{Color: true, LineNumber: 9, HorizontalOffset: 100, StreamFlag: true, StreamNum: 3,
			DID: 0x61, SDID: 0x02, UserData: []uint16{0x101, 0x102, 0x203, 0x104}},
		{LineNumber: 10, DID: 0x41, SDID: 0x05, UserData: []uint16{0x2AA, 0x155, 0x3FF, 0x000, 0x1AB}},
	}
	h := PayloadHeader{ExtendedSequenceNumber: 0xABCD, F: FFirstField}
	payload, err := Marshal(h, in)
	if err != nil {
		t.Fatal(err)
	}
	gotH, gotPkts, err := Unmarshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if gotH != h {
		t.Errorf("header = %+v, want %+v", gotH, h)
	}
	if !reflect.DeepEqual(gotPkts, in) {
		t.Fatalf("packets:\n got %+v\nwant %+v", gotPkts, in)
	}
}

func TestKeepAliveZeroCount(t *testing.T) {
	// ST 2110-40 §5.5: a keep-alive packet has ANC_Count 0 and Length 0.
	payload, err := Marshal(PayloadHeader{ExtendedSequenceNumber: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) != payloadHeaderLen {
		t.Fatalf("len = %d, want %d", len(payload), payloadHeaderLen)
	}
	if payload[2] != 0 || payload[3] != 0 || payload[4] != 0 {
		t.Errorf("Length/ANC_Count not zero: % x", payload[2:5])
	}
	_, pkts, err := Unmarshal(payload)
	if err != nil || len(pkts) != 0 {
		t.Errorf("Unmarshal keep-alive: %d pkts, err %v", len(pkts), err)
	}
}

func TestChecksumValidOnWire(t *testing.T) {
	// The Checksum_Word written for a packet must validate against a recompute.
	p := Packet{DID: 0x61, SDID: 0x02, UserData: []uint16{0x101, 0x102}}
	words := p.tenBitWords()
	cs := words[len(words)-1]
	recomputed := Checksum10(words[:len(words)-1]...)
	if cs != recomputed {
		t.Errorf("on-wire checksum %#x != recomputed %#x", cs, recomputed)
	}
}
