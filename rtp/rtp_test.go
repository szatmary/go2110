package rtp

import (
	"bytes"
	"reflect"
	"testing"
)

func TestHeaderRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		hdr  Header
		want []byte
	}{
		{
			name: "minimal marker PT96",
			hdr: Header{
				Version: 2, Marker: true, PayloadType: 96,
				SequenceNumber: 0x0011, Timestamp: 0x00112233, SSRC: 0x44556677,
			},
			want: []byte{0x80, 0xe0, 0x00, 0x11, 0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77},
		},
		{
			name: "two CSRC",
			hdr: Header{
				Version: 2, PayloadType: 112, SequenceNumber: 1, Timestamp: 90000, SSRC: 0xdeadbeef,
				CSRC: []uint32{0xaabbccdd, 0x11223344},
			},
			want: []byte{
				0x82, 0x70, 0x00, 0x01, 0x00, 0x01, 0x5f, 0x90, 0xde, 0xad, 0xbe, 0xef,
				0xaa, 0xbb, 0xcc, 0xdd, 0x11, 0x22, 0x33, 0x44,
			},
		},
		{
			name: "one-byte header extension",
			hdr: Header{
				Version: 2, Extension: true, PayloadType: 112,
				SequenceNumber: 7, Timestamp: 0, SSRC: 1,
				ExtensionProfile: ExtProfileOneByte,
				ExtensionData:    []byte{0x10, 0xaa, 0x00, 0x00}, // ID 1 len 1 value 0xAA + pad
			},
			want: []byte{
				0x90, 0x70, 0x00, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
				0xbe, 0xde, 0x00, 0x01, 0x10, 0xaa, 0x00, 0x00,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.hdr.Marshal()
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if !bytes.Equal(got, tc.want) {
				t.Fatalf("Marshal = % x, want % x", got, tc.want)
			}
			var h Header
			n, err := h.Unmarshal(got)
			if err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if n != len(tc.want) {
				t.Fatalf("Unmarshal consumed %d, want %d", n, len(tc.want))
			}
			if !reflect.DeepEqual(h, tc.hdr) {
				t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", h, tc.hdr)
			}
		})
	}
}

func TestHeaderBitFields(t *testing.T) {
	h := Header{
		Version: 2, Padding: true, Extension: true, Marker: true,
		PayloadType: 0x7f, SequenceNumber: 0xffff, Timestamp: 0xffffffff, SSRC: 0,
		CSRC:             []uint32{1},
		ExtensionProfile: ExtProfileOneByte, ExtensionData: []byte{0, 0, 0, 0},
	}
	b, err := h.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	// V=2 (10), P=1, X=1, CC=1 -> 1011 0001 = 0xB1
	if b[0] != 0xB1 {
		t.Errorf("byte0 = %#x, want 0xB1", b[0])
	}
	// M=1, PT=0x7f -> 1111 1111 = 0xFF
	if b[1] != 0xFF {
		t.Errorf("byte1 = %#x, want 0xFF", b[1])
	}
}

func TestPacketPadding(t *testing.T) {
	p := Packet{
		Header:      Header{Version: 2, Padding: true, PayloadType: 96, SequenceNumber: 5},
		Payload:     []byte{0xde, 0xad},
		PaddingSize: 4,
	}
	b, err := p.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	// 12 header + 2 payload + 4 padding
	if len(b) != 18 {
		t.Fatalf("len = %d, want 18", len(b))
	}
	if b[len(b)-1] != 4 {
		t.Errorf("padding count octet = %d, want 4", b[len(b)-1])
	}
	for _, pb := range b[14:17] {
		if pb != 0 {
			t.Errorf("padding region not zero: % x", b[14:18])
			break
		}
	}
	var got Packet
	if err := got.Unmarshal(b); err != nil {
		t.Fatal(err)
	}
	if got.PaddingSize != 4 {
		t.Errorf("PaddingSize = %d, want 4", got.PaddingSize)
	}
	if !bytes.Equal(got.Payload, []byte{0xde, 0xad}) {
		t.Errorf("Payload = % x, want de ad", got.Payload)
	}
}

func TestUnmarshalErrors(t *testing.T) {
	var h Header
	if _, err := h.Unmarshal([]byte{0x80, 0x60}); err != ErrShortBuffer {
		t.Errorf("short header: got %v, want ErrShortBuffer", err)
	}
	// Wrong version (3 -> top bits 11).
	bad := []byte{0xC0, 0x60, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	if _, err := h.Unmarshal(bad); err != ErrBadVersion {
		t.Errorf("bad version: got %v, want ErrBadVersion", err)
	}
	// CC says 1 but no CSRC bytes present.
	short := []byte{0x81, 0x60, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	if _, err := h.Unmarshal(short); err != ErrShortBuffer {
		t.Errorf("short csrc: got %v, want ErrShortBuffer", err)
	}
}

func TestExtensionElementsOneByte(t *testing.T) {
	elems := []ExtensionElement{
		{ID: 1, Value: []byte{0xAA}},
		{ID: 2, Value: []byte{0xBB, 0xCC}},
	}
	data, err := BuildExtensionElements(ExtProfileOneByte, elems)
	if err != nil {
		t.Fatal(err)
	}
	if len(data)%4 != 0 {
		t.Fatalf("ext data len %d not multiple of 4", len(data))
	}
	// ID1 len1: 0x10 0xAA ; ID2 len2: 0x21 0xBB 0xCC ; pad to 8 -> 0x00 *2
	want := []byte{0x10, 0xAA, 0x21, 0xBB, 0xCC, 0x00, 0x00, 0x00}
	if !bytes.Equal(data, want) {
		t.Fatalf("build = % x, want % x", data, want)
	}
	got, err := ParseExtensionElements(ExtProfileOneByte, data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, elems) {
		t.Fatalf("parse = %+v, want %+v", got, elems)
	}
}

func TestExtensionElementsTwoByte(t *testing.T) {
	profile := ExtProfileTwoByteBase
	elems := []ExtensionElement{
		{ID: 1, Value: []byte{}},
		{ID: 255, Value: []byte{0xDE, 0xAD, 0xBE, 0xEF}},
	}
	data, err := BuildExtensionElements(profile, elems)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseExtensionElements(profile, data)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 255 {
		t.Fatalf("two-byte parse = %+v", got)
	}
	if !bytes.Equal(got[1].Value, []byte{0xDE, 0xAD, 0xBE, 0xEF}) {
		t.Fatalf("value = % x", got[1].Value)
	}
}
