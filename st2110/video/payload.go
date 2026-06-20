// Package video implements SMPTE ST 2110-20 "Uncompressed Active Video": the RTP
// payload header (extended sequence number + Sample Row Data headers), pgroup
// packing for the sampling/depth combinations of §6.2, a frame packetizer and
// depacketizer (§6.1, §6.3), and the SDP media-type parameters of §7.
package video

import (
	"encoding/binary"
	"errors"
)

// MaxSRD is the maximum number of Sample Row Data headers permitted in one RTP
// packet (ST 2110-20 §6.1.5 / §6.2.1: "RTP Packets shall not contain more than
// three Sample Row Data Headers").
const MaxSRD = 3

// srdHeaderLen is the wire size of one Sample Row Data header: SRD Length (2) +
// F/RowNumber (2) + C/Offset (2) = 6 octets (ST 2110-20 §6.1.4, Figure 2).
const srdHeaderLen = 6

// extSeqLen is the size of the Extended Sequence Number field (§6.1.4).
const extSeqLen = 2

// Errors for payload header parsing.
var (
	ErrShortPayload  = errors.New("st2110/video: payload too short for header")
	ErrNoSRD         = errors.New("st2110/video: payload header must contain at least one SRD header")
	ErrTooManySRD    = errors.New("st2110/video: more than three SRD headers")
	ErrRowOutOfRange = errors.New("st2110/video: SRD row number exceeds 15 bits")
	ErrOffOutOfRange = errors.New("st2110/video: SRD offset exceeds 15 bits")
)

// SRDHeader is a single Sample Row Data header (ST 2110-20 §6.1.4):
//
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|           SRD Length          |F|     SRD Row Number          |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|C|         SRD Offset          |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
type SRDHeader struct {
	// Length is the number of octets of sample-row data described, a multiple of
	// the pgroup octet size. Zero is permitted only for a lone header (no data).
	Length uint16
	// Field is the F bit: 0 for the first field / progressive, 1 for the second
	// field (or PsF segment selector).
	Field bool
	// RowNumber is the 15-bit zero-based sample row number.
	RowNumber uint16
	// Continuation is the C bit: set when another SRD header follows. It is
	// derived from header position on Marshal and read from the wire on
	// Unmarshal.
	Continuation bool
	// Offset is the 15-bit horizontal position of the first sample in the
	// segment, in full-bandwidth pixels from the left edge.
	Offset uint16
}

// PayloadHeader is the ST 2110-20 RTP payload header: a 16-bit Extended Sequence
// Number followed by one to three SRD headers (§6.1.4).
type PayloadHeader struct {
	// ExtendedSequenceNumber is the high 16 bits of the 32-bit extended RTP
	// sequence number; the RTP header carries the low 16 bits.
	ExtendedSequenceNumber uint16
	// SRDs holds 1..3 Sample Row Data headers.
	SRDs []SRDHeader
}

// MarshalSize returns the wire size of the payload header.
func (p *PayloadHeader) MarshalSize() int {
	return extSeqLen + srdHeaderLen*len(p.SRDs)
}

// Marshal returns the wire encoding of the payload header. The Continuation bit
// of each SRD header is set automatically from its position (set on all but the
// last), per §6.1.4.
func (p *PayloadHeader) Marshal() ([]byte, error) {
	if len(p.SRDs) == 0 {
		return nil, ErrNoSRD
	}
	if len(p.SRDs) > MaxSRD {
		return nil, ErrTooManySRD
	}
	buf := make([]byte, p.MarshalSize())
	binary.BigEndian.PutUint16(buf[0:2], p.ExtendedSequenceNumber)
	off := extSeqLen
	for i, s := range p.SRDs {
		if s.RowNumber > 0x7fff {
			return nil, ErrRowOutOfRange
		}
		if s.Offset > 0x7fff {
			return nil, ErrOffOutOfRange
		}
		binary.BigEndian.PutUint16(buf[off:off+2], s.Length)
		row := s.RowNumber & 0x7fff
		if s.Field {
			row |= 0x8000
		}
		binary.BigEndian.PutUint16(buf[off+2:off+4], row)
		offset := s.Offset & 0x7fff
		if i < len(p.SRDs)-1 { // Continuation: another header follows
			offset |= 0x8000
		}
		binary.BigEndian.PutUint16(buf[off+4:off+6], offset)
		off += srdHeaderLen
	}
	return buf, nil
}

// Unmarshal parses a payload header from the front of buf, returning the number
// of octets consumed. It reads SRD headers while the Continuation bit is set, up
// to MaxSRD.
func (p *PayloadHeader) Unmarshal(buf []byte) (int, error) {
	if len(buf) < extSeqLen+srdHeaderLen {
		return 0, ErrShortPayload
	}
	p.ExtendedSequenceNumber = binary.BigEndian.Uint16(buf[0:2])
	off := extSeqLen
	p.SRDs = p.SRDs[:0]
	for {
		if len(buf) < off+srdHeaderLen {
			return 0, ErrShortPayload
		}
		var s SRDHeader
		s.Length = binary.BigEndian.Uint16(buf[off : off+2])
		row := binary.BigEndian.Uint16(buf[off+2 : off+4])
		s.Field = row&0x8000 != 0
		s.RowNumber = row & 0x7fff
		offset := binary.BigEndian.Uint16(buf[off+4 : off+6])
		s.Continuation = offset&0x8000 != 0
		s.Offset = offset & 0x7fff
		p.SRDs = append(p.SRDs, s)
		off += srdHeaderLen
		if !s.Continuation {
			break
		}
		if len(p.SRDs) >= MaxSRD {
			return 0, ErrTooManySRD
		}
	}
	return off, nil
}

// FullSequenceNumber combines the extended sequence number with the 16-bit RTP
// header sequence number into the 32-bit extended counter (§6.1.4).
func (p *PayloadHeader) FullSequenceNumber(rtpSeq uint16) uint32 {
	return uint32(p.ExtendedSequenceNumber)<<16 | uint32(rtpSeq)
}

// SplitSequenceNumber splits a 32-bit extended sequence number into the low 16
// bits (for the RTP header) and high 16 bits (for the payload header).
func SplitSequenceNumber(ext uint32) (low uint16, high uint16) {
	return uint16(ext), uint16(ext >> 16)
}
