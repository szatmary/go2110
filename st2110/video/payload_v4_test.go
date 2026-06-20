package video

import "testing"

// TestV4_ZeroLengthSRD pins ST 2110-20 §6.1.4: a Length-0 SRD is permitted only
// as a lone header. The buggy Marshal accepted a zero-length segment inside a
// multi-SRD packet, which has no valid interpretation.
func TestV4_ZeroLengthSRD(t *testing.T) {
	bad := PayloadHeader{SRDs: []SRDHeader{
		{Length: 10, RowNumber: 0},
		{Length: 0, RowNumber: 1},
	}}
	if _, err := bad.Marshal(); err != ErrZeroLength {
		t.Fatalf("Marshal of multi-SRD with a zero Length = %v, want ErrZeroLength", err)
	}
	// A single zero-length SRD header (no data) is valid.
	ok := PayloadHeader{SRDs: []SRDHeader{{Length: 0, RowNumber: 0}}}
	if _, err := ok.Marshal(); err != nil {
		t.Fatalf("lone zero-length SRD should marshal, got %v", err)
	}
}
