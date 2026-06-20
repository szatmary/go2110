package video

import (
	"bytes"
	"testing"
)

// TestV6_SampleOrderVectors pins the RFC 4175 / ST 2110-20 §6.2 pgroup sample
// orders end-to-end (plane content -> sample order -> bit packing) for the
// packings the suite previously had no on-wire vector for. Using 8-bit depth
// makes each sample one octet, so a scrambled sample order is directly visible.
func TestV6_SampleOrderVectors(t *testing.T) {
	t.Run("YCbCr444", func(t *testing.T) {
		// Order Cb-Y-Cr per pixel (RFC 4175 §6.2). Width 2, Height 1.
		f := Format{Sampling: SamplingYCbCr444, Depth: Depth8, Width: 2, Height: 1}
		fr := NewFrame(f)
		fr.Planes[0] = []uint16{0x11, 0x22} // Cb
		fr.Planes[1] = []uint16{0x33, 0x44} // Y
		fr.Planes[2] = []uint16{0x55, 0x66} // Cr
		want := []byte{0x11, 0x33, 0x55, 0x22, 0x44, 0x66}
		assertLine0(t, fr, want)
	})
	t.Run("RGB", func(t *testing.T) {
		// Order R-G-B per pixel (RFC 4175 §6.2: "Red-Green-Blue").
		f := Format{Sampling: SamplingRGB, Depth: Depth8, Width: 2, Height: 1}
		fr := NewFrame(f)
		fr.Planes[0] = []uint16{0xA0, 0xA1} // R
		fr.Planes[1] = []uint16{0xB0, 0xB1} // G
		fr.Planes[2] = []uint16{0xC0, 0xC1} // B
		want := []byte{0xA0, 0xB0, 0xC0, 0xA1, 0xB1, 0xC1}
		assertLine0(t, fr, want)
	})
	t.Run("Key", func(t *testing.T) {
		// Single component K in raster order.
		f := Format{Sampling: SamplingKey, Depth: Depth8, Width: 4, Height: 1}
		fr := NewFrame(f)
		fr.Planes[0] = []uint16{0x01, 0x02, 0x03, 0x04}
		want := []byte{0x01, 0x02, 0x03, 0x04}
		assertLine0(t, fr, want)
	})
	t.Run("YCbCr420", func(t *testing.T) {
		// Order Y00-Y01-Y10-Y11-Cb-Cr per 2x2 block (RFC 4175 Figure 3).
		// Width 4, Height 2 -> one row-pair, two 2x2 blocks.
		f := Format{Sampling: SamplingYCbCr420, Depth: Depth8, Width: 4, Height: 2}
		fr := NewFrame(f)
		fr.Planes[1] = []uint16{ // Y, 4x2 raster
			0x10, 0x11, 0x12, 0x13,
			0x20, 0x21, 0x22, 0x23,
		}
		fr.Planes[0] = []uint16{0x30, 0x31} // Cb, 2x1
		fr.Planes[2] = []uint16{0x40, 0x41} // Cr, 2x1
		want := []byte{
			0x10, 0x11, 0x20, 0x21, 0x30, 0x40, // block 0
			0x12, 0x13, 0x22, 0x23, 0x31, 0x41, // block 1
		}
		assertLine0(t, fr, want)
	})
}

func assertLine0(t *testing.T, fr *Frame, want []byte) {
	t.Helper()
	pf, err := fr.Pack()
	if err != nil {
		t.Fatal(err)
	}
	if len(pf.Lines) == 0 {
		t.Fatal("no packed lines")
	}
	if !bytes.Equal(pf.Lines[0].Data, want) {
		t.Fatalf("packed line = % x, want % x", pf.Lines[0].Data, want)
	}
	back, err := pf.Unpack()
	if err != nil {
		t.Fatal(err)
	}
	for i := range fr.Planes {
		for j := range fr.Planes[i] {
			if back.Planes[i][j] != fr.Planes[i][j] {
				t.Fatalf("round-trip plane %d[%d] = %#x, want %#x", i, j, back.Planes[i][j], fr.Planes[i][j])
			}
		}
	}
}
