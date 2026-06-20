package timing21

import (
	"math"
	"testing"

	"github.com/szatmary/go2110/st2110/media"
)

// TestP4_ReadScheduleKeepsTROFFSET pins ST 2110-21 §6.4: TPR_j = (j × TRS) + TVD
// with TVD carrying TROFFSET. The buggy ReadSchedule dropped TROFFSET, so the
// whole schedule was un-shifted.
func TestP4_ReadScheduleKeepsTROFFSET(t *testing.T) {
	p := Params{FrameRate: media.Rational{Num: 50, Den: 1}, NPackets: 4, TROffset: 0.001}
	// TFRAME = 0.02 s; linear TRS = TFRAME/N = 0.005 s; sched = TROFFSET + j*TRS.
	sched := p.ReadSchedule(4, Linear)
	want := []float64{0.001, 0.006, 0.011, 0.016}
	for j := range want {
		if math.Abs(sched[j]-want[j]) > 1e-12 {
			t.Fatalf("sched[%d] = %v, want %v (TROFFSET dropped?)", j, sched[j], want[j])
		}
	}
}

// TestP4_InterlacedTRODEFAULT pins ST 2110-21 §6.3.3 Table 1 (1125-line
// interlaced/PsF, BT.709-6): TRODEFAULT = INT((TotalLines − Height)/2)/TotalLines
// × TFRAME. The buggy code did not implement interlaced TRODEFAULT and returned
// the progressive value (43/1125 × TFRAME) for the same height.
func TestP4_InterlacedTRODEFAULT(t *testing.T) {
	// 1080i25: INT((1125-1080)/2) = INT(22.5) = 22; TFRAME = 1/25 = 0.04.
	p := Params{FrameRate: media.Rational{Num: 25, Den: 1}, Height: 1080, Interlaced: true, TotalLines: 1125}
	want := 22.0 / 1125.0 * 0.04
	got := p.TROffsetDefaultSeconds()
	if math.Abs(got-want) > 1e-15 {
		t.Fatalf("interlaced TRODEFAULT = %v, want %v", got, want)
	}
	// Must differ from the progressive value the buggy code returned.
	prog := 43.0 / 1125.0 * 0.04
	if math.Abs(got-prog) < 1e-15 {
		t.Fatal("interlaced TRODEFAULT must not equal the progressive 43/1125 value")
	}
}
