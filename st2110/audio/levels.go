package audio

// Level is an ST 2110-30 Clause 7 conformance level (Table 2 / Table 3).
type Level string

// Conformance levels defined in ST 2110-30 §7, Table 2 (senders).
const (
	LevelA  Level = "A"  // 48 kHz, 1 ms,  1..8  channels (AES67)
	LevelAX Level = "AX" // 96 kHz, 1 ms,  1..4  channels
	LevelB  Level = "B"  // 48 kHz, 125 µs, 1..8  channels
	LevelBX Level = "BX" // 96 kHz, 125 µs, 1..8  channels
	LevelC  Level = "C"  // 48 kHz, 125 µs, 9..64 channels
	LevelCX Level = "CX" // 96 kHz, 125 µs, 9..32 channels
)

// levelSpec is one row of ST 2110-30 §7 Table 2 (Senders Conformance Levels).
type levelSpec struct {
	level        Level
	rateHz       int
	ptimeUs      int64
	minCh, maxCh int
}

// table2 is ST 2110-30:2025 §7 Table 2 — Senders Conformance Levels, verbatim.
var table2 = []levelSpec{
	{LevelA, 48000, 1000, 1, 8},
	{LevelAX, 96000, 1000, 1, 4},
	{LevelB, 48000, 125, 1, 8},
	{LevelBX, 96000, 125, 1, 8},
	{LevelC, 48000, 125, 9, 64},
	{LevelCX, 96000, 125, 9, 32},
}

// MaxChannels is the largest channel count any ST 2110-30 conformance level
// permits (Level C: 64).
const MaxChannels = 64

// ConformanceLevels returns every ST 2110-30 §7 Table 2 sender conformance level
// the format's (sampling rate, packet time, channel count) tuple satisfies. The
// result is empty for a non-conformant tuple.
func (f Format) ConformanceLevels() []Level {
	ptimeUs := f.PacketTime.Microseconds()
	var out []Level
	for _, s := range table2 {
		if f.SampleRate == s.rateHz && ptimeUs == s.ptimeUs && f.Channels >= s.minCh && f.Channels <= s.maxCh {
			out = append(out, s.level)
		}
	}
	return out
}

// Conformant reports whether the format matches at least one ST 2110-30 §7
// Table 2 conformance level.
func (f Format) Conformant() bool {
	return len(f.ConformanceLevels()) > 0
}
