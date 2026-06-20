package video

import (
	"bytes"
	"reflect"
	"testing"
)

func TestPayloadHeaderTwoSRD(t *testing.T) {
	ph := PayloadHeader{
		ExtendedSequenceNumber: 0x0102,
		SRDs: []SRDHeader{
			{Length: 10, RowNumber: 5, Offset: 0},
			{Length: 10, RowNumber: 6, Offset: 0},
		},
	}
	want := []byte{
		0x01, 0x02, // extended sequence number
		0x00, 0x0A, 0x00, 0x05, 0x80, 0x00, // SRD1: len 10, row 5, C=1 off 0
		0x00, 0x0A, 0x00, 0x06, 0x00, 0x00, // SRD2: len 10, row 6, C=0 off 0
	}
	got, err := ph.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("Marshal = % x, want % x", got, want)
	}

	var rt PayloadHeader
	n, err := rt.Unmarshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(want) {
		t.Fatalf("consumed %d, want %d", n, len(want))
	}
	// On the wire SRD1 carries the continuation bit; SRD2 does not.
	wantSRDs := []SRDHeader{
		{Length: 10, RowNumber: 5, Offset: 0, Continuation: true},
		{Length: 10, RowNumber: 6, Offset: 0, Continuation: false},
	}
	if rt.ExtendedSequenceNumber != 0x0102 || !reflect.DeepEqual(rt.SRDs, wantSRDs) {
		t.Fatalf("Unmarshal = %+v", rt)
	}
}

func TestPayloadHeaderFieldBit(t *testing.T) {
	ph := PayloadHeader{
		ExtendedSequenceNumber: 0,
		SRDs:                   []SRDHeader{{Length: 4, Field: true, RowNumber: 0x7fff, Offset: 0x7fff}},
	}
	b, err := ph.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	// Layout: extseq[0:2], Length[2:4], F/row[4:6], C/offset[6:8].
	// F bit + row 0x7fff -> 0xFFFF; single SRD so C=0, offset 0x7fff -> 0x7FFF.
	if b[4] != 0xFF || b[5] != 0xFF {
		t.Errorf("F/row bytes = % x, want FF FF", b[4:6])
	}
	if b[6] != 0x7F || b[7] != 0xFF {
		t.Errorf("C/off bytes = % x, want 7F FF", b[6:8])
	}
}

func TestSequenceNumberSplit(t *testing.T) {
	low, high := SplitSequenceNumber(0xDEADBEEF)
	if low != 0xBEEF || high != 0xDEAD {
		t.Fatalf("split = %#x %#x", low, high)
	}
	ph := PayloadHeader{ExtendedSequenceNumber: high}
	if got := ph.FullSequenceNumber(low); got != 0xDEADBEEF {
		t.Fatalf("combine = %#x", got)
	}
}

func TestPayloadHeaderTooMany(t *testing.T) {
	ph := PayloadHeader{SRDs: make([]SRDHeader, 4)}
	if _, err := ph.Marshal(); err != ErrTooManySRD {
		t.Fatalf("got %v, want ErrTooManySRD", err)
	}
}
