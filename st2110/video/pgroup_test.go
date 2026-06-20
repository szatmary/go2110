package video

import (
	"bytes"
	"testing"
)

func TestPgroupTable(t *testing.T) {
	tests := []struct {
		s    Sampling
		d    Depth
		want Pgroup
	}{
		// 4:2:2 (Table 2)
		{SamplingYCbCr422, Depth8, Pgroup{4, 2, 4}},
		{SamplingYCbCr422, Depth10, Pgroup{5, 2, 4}},
		{SamplingYCbCr422, Depth12, Pgroup{6, 2, 4}},
		{SamplingYCbCr422, Depth16, Pgroup{8, 2, 4}},
		// 4:4:4 (Table 1)
		{SamplingYCbCr444, Depth8, Pgroup{3, 1, 3}},
		{SamplingYCbCr444, Depth10, Pgroup{15, 4, 12}},
		{SamplingYCbCr444, Depth12, Pgroup{9, 2, 6}},
		{SamplingRGB, Depth16, Pgroup{6, 1, 3}},
		// 4:2:0 (Table 3)
		{SamplingYCbCr420, Depth8, Pgroup{6, 4, 6}},
		{SamplingYCbCr420, Depth10, Pgroup{15, 8, 12}},
		{SamplingYCbCr420, Depth12, Pgroup{9, 4, 6}},
		// Key (Table 4)
		{SamplingKey, Depth8, Pgroup{1, 1, 1}},
		{SamplingKey, Depth10, Pgroup{5, 4, 4}},
		{SamplingKey, Depth12, Pgroup{3, 2, 2}},
		{SamplingKey, Depth16, Pgroup{2, 1, 1}},
	}
	for _, tc := range tests {
		got, err := PgroupFor(tc.s, tc.d)
		if err != nil {
			t.Errorf("PgroupFor(%s,%s): %v", tc.s, tc.d, err)
			continue
		}
		if got != tc.want {
			t.Errorf("PgroupFor(%s,%s) = %+v, want %+v", tc.s, tc.d, got, tc.want)
		}
	}
	// 4:2:0 has no 16-bit form.
	if _, err := PgroupFor(SamplingYCbCr420, Depth16); err == nil {
		t.Error("expected error for 4:2:0 16-bit")
	}
}

func TestPackSamples422_10bit_Figure3(t *testing.T) {
	// ST 2110-20 Figure 3: a 5-octet 4:2:2 10-bit pgroup, samples Cb,Y0,Cr,Y1
	// packed MSB-first into a contiguous bitstream.
	tests := []struct {
		samples []uint16
		want    []byte
	}{
		{[]uint16{0x155, 0x2AA, 0x155, 0x2AA}, []byte{0x55, 0x6A, 0xA5, 0x56, 0xAA}},
		{[]uint16{0x3FF, 0x000, 0x3FF, 0x000}, []byte{0xFF, 0xC0, 0x0F, 0xFC, 0x00}},
	}
	for _, tc := range tests {
		got := packSamples(tc.samples, 10)
		if !bytes.Equal(got, tc.want) {
			t.Errorf("packSamples(%v) = % x, want % x", tc.samples, got, tc.want)
		}
		back := unpackSamples(got, len(tc.samples), 10)
		for i := range tc.samples {
			if back[i] != tc.samples[i] {
				t.Errorf("unpack[%d] = %#x, want %#x", i, back[i], tc.samples[i])
			}
		}
	}
}

func TestPackSamples8bitIsBytes(t *testing.T) {
	// At 8-bit depth a pgroup is just the sample octets in order.
	s := []uint16{0x12, 0x34, 0x56}
	got := packSamples(s, 8)
	if !bytes.Equal(got, []byte{0x12, 0x34, 0x56}) {
		t.Errorf("got % x", got)
	}
}

func TestPackSamples16bitBigEndian(t *testing.T) {
	s := []uint16{0x1234, 0xABCD}
	got := packSamples(s, 16)
	if !bytes.Equal(got, []byte{0x12, 0x34, 0xAB, 0xCD}) {
		t.Errorf("got % x", got)
	}
}
