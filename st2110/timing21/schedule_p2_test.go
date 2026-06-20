package timing21

import "testing"

// TestP2_DrainGridEpochAnchored pins ST 2110-21 §6.6.1: the Network Compatibility
// bucket drains on a grid of N×TDRAIN anchored to the SMPTE Epoch, on an
// epoch-absolute timebase. The buggy implementation iterated drain ticks from 0,
// so for a realistic epoch-absolute origin it could not terminate; it also fixed
// the grid to time 0 rather than treating the input as epoch-absolute.
func TestP2_DrainGridEpochAnchored(t *testing.T) {
	// A burst of 3 packets at a large epoch-absolute instant: the result must be
	// computed without iterating ~B/tDrain ticks. The buggy code would spin for
	// ~10^12 iterations here and never return.
	const B = 1e9 // ~31 years of seconds, a plausible TAI-since-epoch magnitude
	if got := MaxCInst([]float64{B, B, B}, 1e-3); got != 3 {
		t.Fatalf("epoch-absolute burst peak = %d, want 3", got)
	}

	// The drain grid is anchored to the Epoch: shifting every arrival by a whole
	// number of drain intervals leaves CINST unchanged.
	pattern := []float64{0.3, 0.35, 1.4}
	base := MaxCInst(pattern, 1.0)
	if base != 2 {
		t.Fatalf("baseline peak = %d, want 2", base)
	}
	shifted := make([]float64, len(pattern))
	for i, v := range pattern {
		shifted[i] = v + 5.0 // +5 × tDrain
	}
	if g := MaxCInst(shifted, 1.0); g != base {
		t.Fatalf("epoch-grid shift changed CINST: %d vs %d", g, base)
	}
}
