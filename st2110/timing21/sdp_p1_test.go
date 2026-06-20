package timing21

import (
	"testing"

	"github.com/szatmary/go2110/sdp"
)

// TestP1_TROFFMicroseconds pins ST 2110-21 §8.2: TROFF is "a positive integer
// number of microseconds". The buggy SetTROFF emitted RTP-clock ticks; at a
// 90 kHz clock a 1 ms offset would have been written as 90, not 1000.
func TestP1_TROFFMicroseconds(t *testing.T) {
	fp := sdp.FormatParameters{Format: "112"}
	SetTROFF(&fp, 0.001) // TROFFSET = 1 ms
	if v, _ := fp.Get("TROFF"); v != "1000" {
		t.Fatalf("TROFF = %q, want \"1000\" (microseconds, §8.2)", v)
	}
	got, ok := ParseTROFF(fp)
	if !ok || got != 0.001 {
		t.Fatalf("ParseTROFF = %v %v, want 0.001", got, ok)
	}
	// A sub-microsecond offset rounds to the nearest microsecond.
	fp2 := sdp.FormatParameters{Format: "112"}
	SetTROFF(&fp2, 0.0000764) // 76.4 µs -> 76
	if v, _ := fp2.Get("TROFF"); v != "76" {
		t.Fatalf("TROFF = %q, want \"76\"", v)
	}
}
