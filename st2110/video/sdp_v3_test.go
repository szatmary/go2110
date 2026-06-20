package video

import (
	"testing"

	"github.com/szatmary/go2110/st2110/media"
)

// TestV3_ColorimetryAlwaysEmitted pins ST 2110-20 §7.2: colorimetry is Required.
// The buggy FMTP dropped it when empty, yielding a colorimetry-less fmtp.
func TestV3_ColorimetryAlwaysEmitted(t *testing.T) {
	f := Format{
		Sampling:       SamplingYCbCr422,
		Depth:          Depth10,
		Width:          1920,
		Height:         1080,
		ExactFrameRate: media.Rational{Num: 60, Den: 1},
		// Colorimetry deliberately left empty.
	}
	fp := f.FMTP(96)
	v, ok := fp.Get("colorimetry")
	if !ok {
		t.Fatalf("colorimetry missing from fmtp %q (it is Required by §7.2)", fp.String())
	}
	if v != "BT709" {
		t.Errorf("default colorimetry = %q, want BT709", v)
	}
}
