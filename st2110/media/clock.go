// Package media implements the ST 2110-10 system timing model that is shared by
// every essence type: the RTP clock, the media clock, and the derivation of RTP
// timestamps from a frame/sample index.
//
// ST 2110-10 §7 defines the relationships. Key points implemented here:
//
//   - The RTP timestamp reflects the sampling instant of the essence in the
//     packet (§7.5), expressed in ticks of the RTP clock.
//   - Video uses a 90 kHz RTP clock (ST 2110-20 §6.1.3).
//   - Timestamps advance at regular increments based on the frame rate,
//     truncating to integer values when necessary (§7.6.1). For 60000/1001 fps
//     this yields the well-known alternating 1501/1502-tick frame periods.
//   - For interlaced video the second field is offset by one half of the frame
//     period, truncated to an integer (§7.6.1).
package media

import "math/big"

// VideoClockRate is the RTP clock rate for ST 2110-20 video streams, in Hz
// (ST 2110-20 §6.1.3).
const VideoClockRate = 90000

// Rational is an exact non-negative rational number, used for frame rates and
// pixel aspect ratios. A frame rate of 60000/1001 fps is Rational{60000, 1001}.
type Rational struct {
	Num int64
	Den int64
}

// Clock models a media/RTP clock running at Rate ticks per second. ST 2110-10
// §7.4 requires the RTP clock and media clock to advance at uniform rates.
type Clock struct {
	// Rate is the clock rate in Hz (e.g. VideoClockRate for video).
	Rate uint32
}

// VideoClock returns a Clock at the ST 2110-20 video rate (90 kHz).
func VideoClock() Clock { return Clock{Rate: VideoClockRate} }

// ticksAt returns floor(index * Rate * fps.Den / fps.Num) reduced modulo 2^32,
// the RTP timestamp of the index-th frame period at the given frame rate. The
// computation uses big integers so it is exact for any index and never
// overflows. The epoch-relative zero-offset convention of ST 2110-10 §7.3
// (offset = 0) is assumed.
func (c Clock) ticksAt(index int64, fps Rational) uint32 {
	// value = floor(index * Rate * Den / Num)
	num := new(big.Int).SetInt64(index)
	num.Mul(num, big.NewInt(int64(c.Rate)))
	num.Mul(num, big.NewInt(fps.Den))
	num.Quo(num, big.NewInt(fps.Num)) // truncating division (index >= 0)

	mod := new(big.Int).Lsh(big.NewInt(1), 32)
	num.Mod(num, mod)
	return uint32(num.Uint64())
}

// FrameTimestamp returns the 32-bit RTP timestamp for the progressive frame at
// the given zero-based index, per ST 2110-10 §7.6.1. Successive frames advance
// at regular, truncated increments; for non-integer rates the per-frame delta
// is not constant (see package doc).
func (c Clock) FrameTimestamp(index int64, fps Rational) uint32 {
	return c.ticksAt(index, fps)
}

// FrameDurationTicks returns the timestamp increment between frame index and
// frame index+1: FrameTimestamp(index+1) - FrameTimestamp(index), computed in
// modular arithmetic. For 90 kHz / (60000/1001) it alternates 1501 and 1502.
func (c Clock) FrameDurationTicks(index int64, fps Rational) uint32 {
	return c.ticksAt(index+1, fps) - c.ticksAt(index, fps)
}

// FieldHalfOffset returns the timestamp offset of the temporally second field of
// an interlaced frame relative to the first field: floor(half a frame period in
// ticks) = floor(Rate * Den / (2 * Num)) (ST 2110-10 §7.6.1).
func (c Clock) FieldHalfOffset(fps Rational) uint32 {
	v := new(big.Int).SetInt64(int64(c.Rate))
	v.Mul(v, big.NewInt(fps.Den))
	den := new(big.Int).Mul(big.NewInt(2), big.NewInt(fps.Num))
	v.Quo(v, den)
	mod := new(big.Int).Lsh(big.NewInt(1), 32)
	v.Mod(v, mod)
	return uint32(v.Uint64())
}

// FieldTimestamp returns the RTP timestamp for an interlaced field. index is the
// zero-based frame index; field is 0 for the first (temporally earlier) field
// and 1 for the second. The first field uses the frame timestamp; the second is
// offset by FieldHalfOffset (ST 2110-10 §7.6.1). PsF segments share the frame
// timestamp, so callers transmitting PsF should use FrameTimestamp for both
// segments.
func (c Clock) FieldTimestamp(index int64, field int, fps Rational) uint32 {
	ts := c.FrameTimestamp(index, fps)
	if field != 0 {
		ts += c.FieldHalfOffset(fps)
	}
	return ts
}

// SampleTimestamp returns the RTP timestamp for the sample at the given
// zero-based sample index for an audio (or other linear) stream, per
// ST 2110-10 §7.7: timestamp = sampleIndex modulo 2^32 when the media clock rate
// equals the RTP clock rate (the audio case, where both run at the sample rate).
func (c Clock) SampleTimestamp(sampleIndex int64) uint32 {
	return uint32(sampleIndex)
}
