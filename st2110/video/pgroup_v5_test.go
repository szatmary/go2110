package video

import "testing"

// TestV5_XYZDepthRestriction pins ST 2110-20 Table 1: XYZ exists only at 12-bit
// and 16-bit/16f depth. The buggy table accepted XYZ at 8 and 10 bits.
func TestV5_XYZDepthRestriction(t *testing.T) {
	for _, d := range []Depth{Depth8, Depth10} {
		if _, err := PgroupFor(SamplingXYZ, d); err == nil {
			t.Errorf("PgroupFor(XYZ, %s) should error: XYZ is undefined below 12-bit", d)
		}
	}
	for _, d := range []Depth{Depth12, Depth16, Depth16f} {
		if _, err := PgroupFor(SamplingXYZ, d); err != nil {
			t.Errorf("PgroupFor(XYZ, %s) should be valid, got %v", d, err)
		}
	}
}
