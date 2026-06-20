package media

import "testing"

// TestM1_SampleTimestampHonorsRate pins ST 2110-10 §7.7: the sample timestamp is
// the sampling instant in RTP-clock ticks, floor(sampleIndex × Rate /
// sampleRate). The buggy SampleTimestamp returned uint32(sampleIndex) regardless
// of the clock Rate.
func TestM1_SampleTimestampHonorsRate(t *testing.T) {
	// Audio (ST 2110-30) case: RTP clock == sample rate -> ts == sample index.
	c48 := Clock{Rate: 48000}
	if got := c48.SampleTimestamp(48000, 48000); got != 48000 {
		t.Errorf("48k clock: %d, want 48000", got)
	}
	// Differing rates: a 90 kHz clock timestamping 48 kHz samples scales by Rate.
	c90 := Clock{Rate: 90000}
	cases := []struct {
		idx  int64
		want uint32
	}{
		{8, 15},  // floor(8*90000/48000) = floor(15) = 15
		{1, 1},   // floor(1.875) = 1 (the buggy code returned 1 here too)
		{16, 30}, // floor(16*1.875) = 30
		{9, 16},  // floor(9*1.875) = floor(16.875) = 16, but the buggy code returned 9
	}
	for _, c := range cases {
		if got := c90.SampleTimestamp(c.idx, 48000); got != c.want {
			t.Errorf("90k clock, 48k samples, idx %d: %d, want %d", c.idx, got, c.want)
		}
	}
}
