// Package audio implements SMPTE ST 2110-30 "PCM Digital Audio": the AES67 /
// RFC 3190 linear PCM RTP payload (L16 / L24), packet-time and channel-count
// handling, packetization/depacketization, and the SDP media-type parameters.
//
// ST 2110-30 §6 constrains AES67: the RTP clock rate equals the audio sampling
// rate (§6.1); the Standard UDP Size Limit is used (§6.2.1); channel order is
// signalled with the RFC 3190 channel-order parameter using the SMPTE2110
// convention (§6.2.2). Samples are interleaved by channel and carried as
// two's-complement, most-significant-byte-first integers (RFC 3190).
package audio

import (
	"errors"
	"time"

	"github.com/szatmary/go2110/rtp"
)

// Encoding is the linear-PCM RTP encoding name (RFC 3190 / AES67).
type Encoding string

// Encodings supported by ST 2110-30 (AES67 mandates L16 and L24).
const (
	L16 Encoding = "L16" // 16-bit linear PCM
	L24 Encoding = "L24" // 24-bit linear PCM
)

// Bytes returns the number of octets per sample for the encoding.
func (e Encoding) Bytes() int {
	switch e {
	case L16:
		return 2
	case L24:
		return 3
	}
	return 0
}

// Packet times defined by ST 2110-30 §7 Table 2: only 1 ms and 125 µs are
// conformant. (250 µs and 1/3 ms are NOT in the standard's allowed set and have
// been removed.)
const (
	PacketTime1ms   = time.Millisecond       // 1000 µs (Levels A, AX)
	PacketTime125us = 125 * time.Microsecond // 125 µs (Levels B, BX, C, CX)
)

// Errors for audio packing.
var (
	ErrUnsupportedEncoding = errors.New("st2110/audio: unsupported encoding")
	ErrBadChannelCount     = errors.New("st2110/audio: invalid channel count")
	ErrBadPacketTime       = errors.New("st2110/audio: packet time does not yield an integer sample count")
	ErrSampleCount         = errors.New("st2110/audio: sample slice not a multiple of channel count")
	ErrShortPayload        = errors.New("st2110/audio: payload length not a multiple of the frame size")
	// ErrSampleRate is returned when the sampling rate is not one of the rates of
	// ST 2110-30 §7 Table 2 (48000 or 96000 Hz).
	ErrSampleRate = errors.New("st2110/audio: sampling rate not 48000 or 96000 Hz (ST 2110-30 §7)")
	// ErrNonConformant is returned when the (sampling rate, packet time, channel
	// count) tuple matches no ST 2110-30 §7 Table 2 conformance level.
	ErrNonConformant = errors.New("st2110/audio: (rate, ptime, channels) matches no ST 2110-30 conformance level")
)

// Format describes an ST 2110-30 audio stream.
type Format struct {
	// Encoding is the PCM encoding (L16 or L24).
	Encoding Encoding
	// SampleRate is the audio sampling rate in Hz (48000, 96000, 44100); it is
	// also the RTP clock rate (§6.1).
	SampleRate int
	// Channels is the number of interleaved audio channels.
	Channels int
	// PacketTime is the audio packet duration (e.g. PacketTime1ms).
	PacketTime time.Duration
	// ChannelOrder is the optional RFC 3190 channel-order value (§6.2.2), e.g.
	// "SMPTE2110.(51,ST)". Empty means the channels are Undefined.
	ChannelOrder string
}

// Validate checks that the format is internally consistent AND conformant to an
// ST 2110-30 §7 (Table 2) conformance level: the sampling rate must be 48000 or
// 96000 Hz, the channel count in 1..MaxChannels (64), and the (rate, packet time,
// channels) tuple must match at least one level (A/AX/B/BX/C/CX).
func (f Format) Validate() error {
	if f.Encoding.Bytes() == 0 {
		return ErrUnsupportedEncoding
	}
	if f.Channels <= 0 || f.Channels > MaxChannels {
		return ErrBadChannelCount
	}
	if f.SampleRate <= 0 || f.PacketTime <= 0 {
		return ErrBadPacketTime
	}
	if _, ok := f.samplesPerPacket(); !ok {
		return ErrBadPacketTime
	}
	if f.SampleRate != 48000 && f.SampleRate != 96000 {
		return ErrSampleRate
	}
	if !f.Conformant() {
		return ErrNonConformant
	}
	return nil
}

// samplesPerPacket returns the number of samples per channel in one packet and
// whether the packet time yields an integer count.
func (f Format) samplesPerPacket() (int, bool) {
	// samples = SampleRate * PacketTime / 1s, exact integer required.
	numer := int64(f.SampleRate) * int64(f.PacketTime)
	den := int64(time.Second)
	if numer%den != 0 {
		return 0, false
	}
	return int(numer / den), true
}

// SamplesPerPacket returns the number of samples per channel carried in each RTP
// packet for the format's packet time (e.g. 48 for 48 kHz at 1 ms).
func (f Format) SamplesPerPacket() int {
	n, _ := f.samplesPerPacket()
	return n
}

// FrameSize returns the number of octets for one sample period across all
// channels (Channels × bytes-per-sample).
func (f Format) FrameSize() int {
	return f.Channels * f.Encoding.Bytes()
}

// PacketPayloadSize returns the RTP payload size in octets for a full packet:
// SamplesPerPacket × Channels × bytes-per-sample.
func (f Format) PacketPayloadSize() int {
	return f.SamplesPerPacket() * f.FrameSize()
}

// PackSamples serializes interleaved PCM samples (channel-interleaved, one int32
// per sample, right-justified two's-complement) into the RTP payload octets,
// most-significant-byte-first per RFC 3190. len(samples) must be a multiple of
// Channels.
func (f Format) PackSamples(samples []int32) ([]byte, error) {
	if f.Encoding.Bytes() == 0 {
		return nil, ErrUnsupportedEncoding
	}
	if f.Channels <= 0 || len(samples)%f.Channels != 0 {
		return nil, ErrSampleCount
	}
	bps := f.Encoding.Bytes()
	out := make([]byte, len(samples)*bps)
	o := 0
	for _, s := range samples {
		u := uint32(s)
		switch bps {
		case 2:
			out[o] = byte(u >> 8)
			out[o+1] = byte(u)
		case 3:
			out[o] = byte(u >> 16)
			out[o+1] = byte(u >> 8)
			out[o+2] = byte(u)
		}
		o += bps
	}
	return out, nil
}

// UnpackSamples is the inverse of PackSamples, sign-extending each sample to
// int32. len(data) must be a multiple of bytes-per-sample.
func (f Format) UnpackSamples(data []byte) ([]int32, error) {
	bps := f.Encoding.Bytes()
	if bps == 0 {
		return nil, ErrUnsupportedEncoding
	}
	if len(data)%bps != 0 {
		return nil, ErrShortPayload
	}
	out := make([]int32, len(data)/bps)
	for i := range out {
		o := i * bps
		switch bps {
		case 2:
			out[i] = int32(int16(uint16(data[o])<<8 | uint16(data[o+1])))
		case 3:
			v := uint32(data[o])<<16 | uint32(data[o+1])<<8 | uint32(data[o+2])
			if v&0x800000 != 0 { // sign-extend 24-bit
				v |= 0xff000000
			}
			out[i] = int32(v)
		}
	}
	return out, nil
}

// PacketizeOptions controls audio RTP packetization.
type PacketizeOptions struct {
	PayloadType uint8
	SSRC        uint32
	// StartTimestamp is the RTP timestamp of the first sample; it increments by
	// SamplesPerPacket for each packet (ST 2110-10 §7.7).
	StartTimestamp uint32
	// StartSequence is the RTP sequence number of the first packet.
	StartSequence uint16
}

// Packetize splits a contiguous run of interleaved samples into RTP packets of
// SamplesPerPacket sample periods each. The number of samples must be a multiple
// of Channels; any trailing partial packet (fewer than SamplesPerPacket sample
// periods) is emitted as a final shorter packet.
func (f Format) Packetize(samples []int32, opts PacketizeOptions) ([]rtp.Packet, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	if len(samples)%f.Channels != 0 {
		return nil, ErrSampleCount
	}
	spp := f.SamplesPerPacket()
	periodsTotal := len(samples) / f.Channels

	var packets []rtp.Packet
	seq := opts.StartSequence
	ts := opts.StartTimestamp
	for start := 0; start < periodsTotal; start += spp {
		n := spp
		if start+n > periodsTotal {
			n = periodsTotal - start
		}
		chunk := samples[start*f.Channels : (start+n)*f.Channels]
		payload, err := f.PackSamples(chunk)
		if err != nil {
			return nil, err
		}
		packets = append(packets, rtp.Packet{
			Header: rtp.Header{
				Version:        rtp.Version,
				PayloadType:    opts.PayloadType,
				SequenceNumber: seq,
				Timestamp:      ts,
				SSRC:           opts.SSRC,
			},
			Payload: payload,
		})
		seq++
		ts += uint32(n)
	}
	return packets, nil
}

// Depacketize concatenates the interleaved samples from a sequence of RTP audio
// packets (assumed already ordered) into a single sample slice.
func (f Format) Depacketize(packets []rtp.Packet) ([]int32, error) {
	if f.Encoding.Bytes() == 0 {
		return nil, ErrUnsupportedEncoding
	}
	var out []int32
	for i := range packets {
		s, err := f.UnpackSamples(packets[i].Payload)
		if err != nil {
			return nil, err
		}
		out = append(out, s...)
	}
	return out, nil
}
