package rtp

import (
	"bytes"
	"reflect"
	"testing"
)

// TestS1_TwoByteHeaderExtensionVector pins a complete on-wire RTP datagram with
// an RFC 8285 two-byte header extension (profile 0x1000): header marshal, the
// exact byte vector, round-trip, and decoding of the two-byte elements.
func TestS1_TwoByteHeaderExtensionVector(t *testing.T) {
	// Two-byte elements: ID 1 len 1 value 0xAA; ID 2 len 2 value 0xBB 0xCC; the
	// 7-octet element region is zero-padded to the 4-octet (1-word) boundary.
	extData := []byte{0x01, 0x01, 0xAA, 0x02, 0x02, 0xBB, 0xCC, 0x00}
	hdr := Header{
		Version: 2, Extension: true, PayloadType: 96,
		SequenceNumber: 9, Timestamp: 0x11223344, SSRC: 0x01020304,
		ExtensionProfile: ExtProfileTwoByteBase, // 0x1000
		ExtensionData:    extData,
	}
	want := []byte{
		0x90, 0x60, 0x00, 0x09, // V=2 X=1 CC=0 | M=0 PT=96 | seq=9
		0x11, 0x22, 0x33, 0x44, // timestamp
		0x01, 0x02, 0x03, 0x04, // SSRC
		0x10, 0x00, 0x00, 0x02, // ext profile 0x1000, length = 2 words
		0x01, 0x01, 0xAA, // ID 1, len 1, 0xAA
		0x02, 0x02, 0xBB, 0xCC, // ID 2, len 2, 0xBB 0xCC
		0x00, // word_align pad
	}
	got, err := hdr.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("Marshal = % x\nwant      % x", got, want)
	}

	var rt Header
	n, err := rt.Unmarshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(want) {
		t.Fatalf("Unmarshal consumed %d, want %d", n, len(want))
	}
	if !reflect.DeepEqual(rt, hdr) {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", rt, hdr)
	}

	// The element region decodes to the two original two-byte elements.
	elems, err := ParseExtensionElements(ExtProfileTwoByteBase, rt.ExtensionData)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 2 ||
		elems[0].ID != 1 || !bytes.Equal(elems[0].Value, []byte{0xAA}) ||
		elems[1].ID != 2 || !bytes.Equal(elems[1].Value, []byte{0xBB, 0xCC}) {
		t.Fatalf("decoded elements = %+v", elems)
	}
}
