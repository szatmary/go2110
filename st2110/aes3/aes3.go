// Package aes3 implements SMPTE ST 2110-31 "AES3 Transparent Transport": the RTP
// carriage of AES3 subframes as 32-bit AM824 subframes, preserving the B, F, and
// P/C/U/V bits and the 24-bit audio data (§5.4). The RTP clock equals the AES3
// sampling frequency (§5.5); the payload is an interleaved sequence of AM824
// subframes, with the period count per packet signalled by ptime (§6.1).
package aes3

import (
	"errors"
	"math"
	"time"

	"github.com/szatmary/go2110/rtp"
)

// SubframeSize is the wire size of one AM824 subframe (ST 2110-31 §5.4): 32 bits.
const SubframeSize = 4

// Errors for AES3 packing.
var (
	ErrBadChannels   = errors.New("st2110/aes3: number of subframe sequences must be a positive even number within the level ceiling")
	ErrBadRate       = errors.New("st2110/aes3: unsupported sampling rate")
	ErrBadPacketTime = errors.New("st2110/aes3: packet time is not a permitted ST 2110-31 Table 1 value for the rate")
	ErrBadPacket     = errors.New("st2110/aes3: payload not a whole number of AM824 subframes")
	ErrSubframeAlign = errors.New("st2110/aes3: subframe count not a multiple of the channel count")
)

// Subframe is one AM824 subframe (ST 2110-31 §5.4, Figure 2):
//
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|0|0|B|F| PCUV |                   DATA24                       |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
type Subframe struct {
	// BlockStart (B) marks the first subframe of an AES3 Block ("Z" preamble).
	BlockStart bool
	// FrameStart (F) marks the first subframe of an AES3 Frame.
	FrameStart bool
	// P is the AES3 parity bit (timeslot 31).
	P bool
	// C is the AES3 channel-status bit (timeslot 30).
	C bool
	// U is the AES3 user-data bit (timeslot 29).
	U bool
	// V is the AES3 validity bit (timeslot 28).
	V bool
	// Data holds the 24-bit AES3 audio sample (timeslots 4..27), right-justified
	// with the most significant bit (timeslot 27) at bit 23.
	Data uint32
}

// Marshal encodes the subframe as 4 big-endian octets.
func (s Subframe) Marshal() [SubframeSize]byte {
	var b0 byte
	if s.BlockStart {
		b0 |= 1 << 5
	}
	if s.FrameStart {
		b0 |= 1 << 4
	}
	if s.P {
		b0 |= 1 << 3
	}
	if s.C {
		b0 |= 1 << 2
	}
	if s.U {
		b0 |= 1 << 1
	}
	if s.V {
		b0 |= 1
	}
	d := s.Data & 0xFFFFFF
	return [SubframeSize]byte{b0, byte(d >> 16), byte(d >> 8), byte(d)}
}

// ParseSubframe decodes a 4-octet AM824 subframe.
func ParseSubframe(b []byte) Subframe {
	return Subframe{
		BlockStart: b[0]&(1<<5) != 0,
		FrameStart: b[0]&(1<<4) != 0,
		P:          b[0]&(1<<3) != 0,
		C:          b[0]&(1<<2) != 0,
		U:          b[0]&(1<<1) != 0,
		V:          b[0]&1 != 0,
		Data:       uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]),
	}
}

// Format describes an ST 2110-31 AES3 stream.
type Format struct {
	// SampleRate is the AES3 sampling frequency in Hz (44100, 48000, 96000); it
	// is also the RTP clock rate (§5.5).
	SampleRate int
	// Channels is <nchan>: the number of interleaved AES3 subframe sequences.
	// It must be a positive even number, since each AES3 signal carries two
	// subframe sequences (§6.1).
	Channels int
	// PacketTime is the packet duration signalled via ptime (Table 1).
	PacketTime time.Duration
}

// Validate checks the format against ST 2110-31 §5.5/§6.1 and the permitted
// packet times of Table 1: the sampling rate must be 44100/48000/96000 Hz, the
// subframe-sequence count must be a positive even number not exceeding the
// Table 3 ceiling for the mode, and the packet time must resolve to one of the
// Table 1 (rate, periods) combinations.
func (f Format) Validate() error {
	switch f.SampleRate {
	case 44100, 48000, 96000:
	default:
		return ErrBadRate
	}
	if f.Channels <= 0 || f.Channels%2 != 0 {
		return ErrBadChannels
	}
	m, ok := f.modeFor()
	if !ok {
		return ErrBadPacketTime
	}
	if f.Channels > m.maxSeqs {
		return ErrBadChannels
	}
	return nil
}

// PeriodsPerPacket returns the number of sampling-clock periods carried in each
// RTP packet (Table 1), rounded to the nearest integer (the 44.1 kHz packet
// times are approximate, e.g. 1.09 ms → 48 periods).
func (f Format) PeriodsPerPacket() int {
	return int(math.Round(float64(f.SampleRate) * f.PacketTime.Seconds()))
}

// SubframesPerPacket returns the number of AM824 subframes per packet:
// PeriodsPerPacket × Channels.
func (f Format) SubframesPerPacket() int {
	return f.PeriodsPerPacket() * f.Channels
}

// PacketPayloadSize returns the RTP payload size in octets for a full packet.
func (f Format) PacketPayloadSize() int {
	return f.SubframesPerPacket() * SubframeSize
}

// PackSubframes serializes interleaved AM824 subframes into RTP payload octets.
func PackSubframes(subframes []Subframe) []byte {
	out := make([]byte, len(subframes)*SubframeSize)
	for i, s := range subframes {
		b := s.Marshal()
		copy(out[i*SubframeSize:], b[:])
	}
	return out
}

// UnpackSubframes is the inverse of PackSubframes.
func UnpackSubframes(payload []byte) ([]Subframe, error) {
	if len(payload)%SubframeSize != 0 {
		return nil, ErrBadPacket
	}
	out := make([]Subframe, len(payload)/SubframeSize)
	for i := range out {
		out[i] = ParseSubframe(payload[i*SubframeSize : i*SubframeSize+SubframeSize])
	}
	return out, nil
}

// PacketizeOptions controls AES3 RTP packetization.
type PacketizeOptions struct {
	PayloadType    uint8
	SSRC           uint32
	StartTimestamp uint32
	StartSequence  uint16
}

// Packetize splits a contiguous interleaved subframe sequence into RTP packets
// of PeriodsPerPacket periods each. len(subframes) must be a multiple of
// Channels. The RTP timestamp advances by the period count per packet (§5.3 of
// ST 2110-10, audio rules).
func (f Format) Packetize(subframes []Subframe, opts PacketizeOptions) ([]rtp.Packet, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	if len(subframes)%f.Channels != 0 {
		return nil, ErrSubframeAlign
	}
	chunk := f.PeriodsPerPacket() * f.Channels

	var packets []rtp.Packet
	seq := opts.StartSequence
	ts := opts.StartTimestamp
	for start := 0; start < len(subframes); start += chunk {
		end := start + chunk
		if end > len(subframes) {
			end = len(subframes)
		}
		packets = append(packets, rtp.Packet{
			Header: rtp.Header{
				Version:        rtp.Version,
				PayloadType:    opts.PayloadType,
				SequenceNumber: seq,
				Timestamp:      ts,
				SSRC:           opts.SSRC,
			},
			Payload: PackSubframes(subframes[start:end]),
		})
		seq++
		ts += uint32((end - start) / f.Channels)
	}
	return packets, nil
}

// Depacketize concatenates the interleaved subframes from ordered RTP packets.
func (f Format) Depacketize(packets []rtp.Packet) ([]Subframe, error) {
	var out []Subframe
	for i := range packets {
		s, err := UnpackSubframes(packets[i].Payload)
		if err != nil {
			return nil, err
		}
		out = append(out, s...)
	}
	return out, nil
}
