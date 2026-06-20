package aes3

import (
	"testing"
	"time"
)

// TestE1_PTime0_12 pins ST 2110-31 Table 1: a 48 kHz 125 µs packet is signalled
// as ptime "0.12" (the Table 3 note rounds the midway 0.125 down). The buggy
// PTime emitted "0.125".
func TestE1_PTime0_12(t *testing.T) {
	f := Format{SampleRate: 48000, Channels: 2, PacketTime: 125 * time.Microsecond}
	if got := f.PTime(); got != "0.12" {
		t.Fatalf("PTime = %q, want \"0.12\" (Table 1; 0.125 rounds down)", got)
	}
	// And it appears in the SDP ptime attribute.
	m := f.MediaDescription(50000, 97)
	if v, _ := m.GetAttribute("ptime"); v != "0.12" {
		t.Fatalf("SDP ptime = %q, want \"0.12\"", v)
	}
	// Other Table 1 values: 1 ms -> "1", 80 µs -> "0.08", 44.1k 6-period -> "0.14".
	cases := []struct {
		rate int
		pt   time.Duration
		want string
	}{
		{48000, time.Millisecond, "1"},
		{48000, 80 * time.Microsecond, "0.08"},
		{96000, 125 * time.Microsecond, "0.12"},
		{44100, 6 * time.Second / 44100, "0.14"}, // 6 periods at 44.1 kHz ≈ 136 µs
	}
	for _, c := range cases {
		f := Format{SampleRate: c.rate, Channels: 2, PacketTime: c.pt}
		if got := f.PTime(); got != c.want {
			t.Errorf("%d@%v: PTime = %q, want %q", c.rate, c.pt, got, c.want)
		}
	}
}

// TestE2_ValidatePacketTime pins ST 2110-31 Table 1: Validate accepts only the
// permitted (rate, packet-time) combinations and rejects others.
func TestE2_ValidatePacketTime(t *testing.T) {
	tests := []struct {
		name string
		f    Format
		want error
	}{
		{"48k/1ms ok", Format{SampleRate: 48000, Channels: 2, PacketTime: time.Millisecond}, nil},
		{"48k/125us ok (0.12)", Format{SampleRate: 48000, Channels: 8, PacketTime: 125 * time.Microsecond}, nil},
		{"48k/250us not in Table 1", Format{SampleRate: 48000, Channels: 2, PacketTime: 250 * time.Microsecond}, ErrBadPacketTime},
		{"96k/1ms ok", Format{SampleRate: 96000, Channels: 2, PacketTime: time.Millisecond}, nil},
		{"96k/1ms over (max 2 seqs)", Format{SampleRate: 96000, Channels: 4, PacketTime: time.Millisecond}, ErrBadChannels},
		{"48k/0.12 over D ceiling (60)", Format{SampleRate: 48000, Channels: 62, PacketTime: 125 * time.Microsecond}, ErrBadChannels},
	}
	for _, tc := range tests {
		if err := tc.f.Validate(); err != tc.want {
			t.Errorf("%s: Validate = %v, want %v", tc.name, err, tc.want)
		}
	}
}

// TestE2_ConformanceLevels pins ST 2110-31 §7 Table 3 level membership.
func TestE2_ConformanceLevels(t *testing.T) {
	// 48 kHz / 1 ms / 6 seqs is Level A (and every higher level's 1 ms row).
	a := Format{SampleRate: 48000, Channels: 6, PacketTime: time.Millisecond}
	levels := a.ConformanceLevels()
	if len(levels) == 0 || levels[0] != LevelA {
		t.Fatalf("48k/1ms/6: levels = %v, want to start with A", levels)
	}
	// 48 kHz / 0.12 / 8 seqs is Level B (8 > Level A's 1ms-only scope).
	b := Format{SampleRate: 48000, Channels: 8, PacketTime: 125 * time.Microsecond}
	if !contains(b.ConformanceLevels(), LevelB) {
		t.Errorf("48k/0.12/8 should include Level B: %v", b.ConformanceLevels())
	}
	// 48 kHz / 0.08 / 80 seqs is Level D only.
	d := Format{SampleRate: 48000, Channels: 80, PacketTime: 80 * time.Microsecond}
	dl := d.ConformanceLevels()
	if !contains(dl, LevelD) || contains(dl, LevelC) {
		t.Errorf("48k/0.08/80 should be Level D (not C): %v", dl)
	}
	// 96 kHz / 0.12 / 30 seqs is Level CX/DX.
	cx := Format{SampleRate: 96000, Channels: 30, PacketTime: 125 * time.Microsecond}
	if !contains(cx.ConformanceLevels(), LevelCX) {
		t.Errorf("96k/0.12/30 should include Level CX: %v", cx.ConformanceLevels())
	}
}

func contains(ls []Level, l Level) bool {
	for _, x := range ls {
		if x == l {
			return true
		}
	}
	return false
}
