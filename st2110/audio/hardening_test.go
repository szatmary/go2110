package audio

import (
	"testing"
	"time"
)

// TestA1_ConformanceLevels pins ST 2110-30 §7 Table 2 (Senders): the level a
// given (rate, ptime, channels) tuple satisfies, at the level boundaries.
func TestA1_ConformanceLevels(t *testing.T) {
	cases := []struct {
		rate     int
		pt       time.Duration
		channels int
		want     Level
	}{
		{48000, PacketTime1ms, 1, LevelA},     // A lower bound
		{48000, PacketTime1ms, 8, LevelA},     // A upper bound
		{96000, PacketTime1ms, 4, LevelAX},    // AX upper bound
		{48000, PacketTime125us, 8, LevelB},   // B upper bound
		{48000, PacketTime125us, 9, LevelC},   // C lower bound
		{48000, PacketTime125us, 64, LevelC},  // C upper bound
		{96000, PacketTime125us, 8, LevelBX},  // BX upper bound
		{96000, PacketTime125us, 9, LevelCX},  // CX lower bound
		{96000, PacketTime125us, 32, LevelCX}, // CX upper bound
	}
	for _, c := range cases {
		f := Format{Encoding: L24, SampleRate: c.rate, Channels: c.channels, PacketTime: c.pt}
		got := f.ConformanceLevels()
		if len(got) != 1 || got[0] != c.want {
			t.Errorf("%d/%dus/%dch: levels = %v, want [%s]", c.rate, c.pt.Microseconds(), c.channels, got, c.want)
		}
	}
}

// TestA1_ValidateRejectsOutOfSpec pins that Validate rejects tuples outside the
// Table 2 ranges (the buggy Validate accepted any positive integer-sample tuple).
func TestA1_ValidateRejectsOutOfSpec(t *testing.T) {
	tests := []struct {
		name string
		f    Format
		want error
	}{
		{"A 8ch ok", Format{Encoding: L24, SampleRate: 48000, Channels: 8, PacketTime: PacketTime1ms}, nil},
		{"A 9ch over (no 48k/1ms level >8)", Format{Encoding: L24, SampleRate: 48000, Channels: 9, PacketTime: PacketTime1ms}, ErrNonConformant},
		{"AX 4ch ok", Format{Encoding: L24, SampleRate: 96000, Channels: 4, PacketTime: PacketTime1ms}, nil},
		{"AX 5ch over (96k/1ms max 4)", Format{Encoding: L24, SampleRate: 96000, Channels: 5, PacketTime: PacketTime1ms}, ErrNonConformant},
		{"C 64ch ok", Format{Encoding: L24, SampleRate: 48000, Channels: 64, PacketTime: PacketTime125us}, nil},
		{"65ch over MaxChannels", Format{Encoding: L24, SampleRate: 48000, Channels: 65, PacketTime: PacketTime125us}, ErrBadChannelCount},
		{"CX 33ch over (96k/125us max 32)", Format{Encoding: L24, SampleRate: 96000, Channels: 33, PacketTime: PacketTime125us}, ErrNonConformant},
		{"32kHz not in spec set", Format{Encoding: L24, SampleRate: 32000, Channels: 2, PacketTime: PacketTime1ms}, ErrSampleRate},
	}
	for _, tc := range tests {
		if err := tc.f.Validate(); err != tc.want {
			t.Errorf("%s: Validate = %v, want %v", tc.name, err, tc.want)
		}
	}
}

// TestA2_BogusPacketTimesRemoved pins ST 2110-30 §7 Table 2: only 1 ms and 125 µs
// are allowed. A 250 µs / 333 µs packet time is non-conformant (the removed
// PacketTime250us/333us constants were not in Table 2).
func TestA2_BogusPacketTimesRemoved(t *testing.T) {
	for _, ptUs := range []int64{250, 333} {
		f := Format{Encoding: L24, SampleRate: 48000, Channels: 2, PacketTime: time.Duration(ptUs) * time.Microsecond}
		if f.Conformant() {
			t.Errorf("%d µs packet time should be non-conformant (not in Table 2)", ptUs)
		}
		if err := f.Validate(); err == nil {
			t.Errorf("%d µs packet time should fail Validate", ptUs)
		}
	}
	// The two conformant packet times remain valid.
	if PacketTime1ms.Microseconds() != 1000 || PacketTime125us.Microseconds() != 125 {
		t.Fatal("conformant packet-time constants changed")
	}
}
