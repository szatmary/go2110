package aes3

// Level is an ST 2110-31 §7 (Table 3) conformance level.
type Level string

// Conformance levels defined in ST 2110-31 §7, Table 3.
const (
	LevelA  Level = "A"
	LevelAX Level = "AX"
	LevelB  Level = "B"
	LevelBX Level = "BX"
	LevelC  Level = "C"
	LevelCX Level = "CX"
	LevelD  Level = "D"
	LevelDX Level = "DX"
)

// mode is one permitted (clock rate, packet time) combination of ST 2110-31
// Table 1, keyed by the number of RTP-clock periods per packet (which uniquely
// identifies the packet time for a given rate). maxSeqs is the largest number of
// AES3 Subframe sequences any conformance level permits for the mode (Table 3,
// Level D/DX).
type mode struct {
	rate    int
	periods int
	ptime   string
	maxSeqs int
}

// table1 is ST 2110-31 Table 1 — Permitted values of Packet Time for each RTP
// Clock rate, with the absolute Subframe-sequence ceiling from Table 3.
var table1 = []mode{
	{48000, 48, "1", 6},
	{48000, 6, "0.12", 60},
	{48000, 4, "0.08", 80},
	{96000, 96, "1", 2},
	{96000, 12, "0.12", 30},
	{96000, 8, "0.08", 40},
	{44100, 48, "1.09", 6},
	{44100, 6, "0.14", 60},
	{44100, 4, "0.09", 80},
}

// levelRow is one (level, rate, periods, max-subframe-sequences) entry of
// ST 2110-31 §7 Table 3.
type levelRow struct {
	level   Level
	rate    int
	periods int
	maxSeqs int
}

// table3 is ST 2110-31:2022 §7 Table 3 — Conformance Levels, transcribed
// verbatim. periods identify the signalled packet time (48000: 1ms→48,
// 0.12→6, 0.08→4; 44100: 1.09→48, 0.14→6, 0.09→4; 96000: 1→96, 0.12→12, 0.08→8).
var table3 = []levelRow{
	{LevelA, 48000, 48, 6},

	{LevelAX, 48000, 48, 6}, {LevelAX, 44100, 48, 6}, {LevelAX, 96000, 96, 2},

	{LevelB, 48000, 48, 6}, {LevelB, 48000, 6, 8},

	{LevelBX, 48000, 48, 6}, {LevelBX, 48000, 6, 8},
	{LevelBX, 44100, 48, 6}, {LevelBX, 44100, 6, 8},
	{LevelBX, 96000, 96, 2}, {LevelBX, 96000, 12, 4},

	{LevelC, 48000, 48, 6}, {LevelC, 48000, 6, 60},

	{LevelCX, 48000, 48, 6}, {LevelCX, 48000, 6, 60},
	{LevelCX, 44100, 48, 6}, {LevelCX, 44100, 6, 60},
	{LevelCX, 96000, 96, 2}, {LevelCX, 96000, 12, 30},

	{LevelD, 48000, 48, 6}, {LevelD, 48000, 6, 60}, {LevelD, 48000, 4, 80},

	{LevelDX, 48000, 48, 6}, {LevelDX, 48000, 6, 60}, {LevelDX, 48000, 4, 80},
	{LevelDX, 44100, 48, 6}, {LevelDX, 44100, 6, 60}, {LevelDX, 44100, 4, 80},
	{LevelDX, 96000, 96, 2}, {LevelDX, 96000, 12, 30}, {LevelDX, 96000, 8, 40},
}

// modeFor returns the Table 1 mode matching the format's rate and periods.
func (f Format) modeFor() (mode, bool) {
	periods := f.PeriodsPerPacket()
	for _, m := range table1 {
		if m.rate == f.SampleRate && m.periods == periods {
			return m, true
		}
	}
	return mode{}, false
}

// ConformanceLevels returns every ST 2110-31 §7 Table 3 conformance level whose
// permitted combinations include the format's (sampling rate, packet time,
// subframe-sequence count) tuple. The result is empty for a non-conformant tuple.
func (f Format) ConformanceLevels() []Level {
	periods := f.PeriodsPerPacket()
	var out []Level
	seen := map[Level]bool{}
	for _, r := range table3 {
		if r.rate == f.SampleRate && r.periods == periods && f.Channels > 0 && f.Channels <= r.maxSeqs && !seen[r.level] {
			seen[r.level] = true
			out = append(out, r.level)
		}
	}
	return out
}

// Conformant reports whether the format matches at least one ST 2110-31 §7
// Table 3 conformance level.
func (f Format) Conformant() bool {
	return len(f.ConformanceLevels()) > 0
}
