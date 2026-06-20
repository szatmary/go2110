// Package anc implements SMPTE ST 2110-40 "Ancillary Data": the carriage of
// SMPTE ST 291-1 ANC data packets over RTP using the payload format of IETF RFC
// 8331. ST 2110-40 §5 maps ANC packets directly per RFC 8331; the RTP clock is
// 90 kHz (§5.3) and the timestamp is contemporaneous with the related video
// frame/field (§5.4).
//
// The RTP payload begins with an 8-octet header (Extended Sequence Number,
// Length, ANC_Count, F, reserved) followed by ANC_Count ANC data packets. Each
// ANC data packet has a 32-bit location header (C, Line_Number,
// Horizontal_Offset, S, StreamNum) followed by the 10-bit ST 291-1 data words
// (DID, SDID, Data_Count, User_Data_Words, Checksum) and word_align padding.
package anc

import (
	"encoding/binary"
	"errors"
)

// RTPClockRate is the ANC RTP clock rate (ST 2110-40 §5.3): 90 kHz.
const RTPClockRate = 90000

// payloadHeaderLen is the size of the ANC RTP payload header (RFC 8331 §2.1):
// ExtSeq(2) + Length(2) + ANC_Count(1) + F/reserved(3) = 8 octets.
const payloadHeaderLen = 8

// F field values (RFC 8331 §2.1): interlaced field signalling.
const (
	FProgressive uint8 = 0b00 // progressive or no field specified
	FFirstField  uint8 = 0b10 // timestamp refers to the first field
	FSecondField uint8 = 0b11 // timestamp refers to the second field
)

// Special Line_Number values (RFC 8331 §2.1).
const (
	LineNoWithoutLocation uint16 = 0x7FF // no specific line location
	LineNoVANCRange       uint16 = 0x7FE // generic VANC range
)

// Errors for ANC marshaling.
var (
	ErrShortHeader  = errors.New("st2110/anc: payload shorter than the RTP payload header")
	ErrShortPayload = errors.New("st2110/anc: payload shorter than declared ANC data")
	ErrTooManyANC   = errors.New("st2110/anc: more than 255 ANC data packets")
	ErrDataCount    = errors.New("st2110/anc: User_Data_Words count exceeds 255")
	ErrFieldRange   = errors.New("st2110/anc: header field value out of range")
	// ErrInvalidField is returned for an F field value of 0b01, which RFC 8331
	// §2.1 declares "is not valid".
	ErrInvalidField = errors.New("st2110/anc: F field value 0b01 is not valid (RFC 8331 §2.1)")
	// ErrChecksum is returned by Unmarshal when an ANC data packet's
	// Checksum_Word does not match the ST 291-1 checksum of its words.
	ErrChecksum = errors.New("st2110/anc: ANC data packet Checksum_Word mismatch")
)

// Packet is a single SMPTE ST 291-1 ANC data packet carried per RFC 8331 §2.1.
// DID and SDID are the 8-bit identification values (parity is added on the wire);
// UserData holds the 10-bit User_Data_Words as transmitted (low 10 bits used).
type Packet struct {
	// Color is the C bit: ANC is in the color-difference channel when set.
	Color bool
	// LineNumber is the 11-bit digital interface line number.
	LineNumber uint16
	// HorizontalOffset is the 12-bit horizontal offset.
	HorizontalOffset uint16
	// StreamFlag is the S (Data Stream Flag) bit.
	StreamFlag bool
	// StreamNum is the 7-bit data stream number.
	StreamNum uint8
	// DID is the 8-bit Data Identification word value.
	DID uint8
	// SDID is the 8-bit Secondary Data Identification (or DBN) value.
	SDID uint8
	// UserData holds the 10-bit User_Data_Words.
	UserData []uint16
}

// PayloadHeader is the ANC RTP payload header (RFC 8331 §2.1).
type PayloadHeader struct {
	// ExtendedSequenceNumber is the high 16 bits of the extended RTP sequence
	// number (same as RFC 4175 / ST 2110-20).
	ExtendedSequenceNumber uint16
	// F is the field signalling value (FProgressive/FFirstField/FSecondField).
	F uint8
}

// EvenParity10 encodes an 8-bit value as a 10-bit ST 291-1 word: bits b7..b0 are
// the value, bit b8 is the even parity of b7..b0, and bit b9 is the inverse of
// b8 (RFC 8331 §2.1, e.g. for DID/SDID/Data_Count).
func EvenParity10(v uint8) uint16 {
	b8 := uint16(0)
	x := v
	for x != 0 { // XOR of all data bits = even-parity bit
		b8 ^= 1
		x &= x - 1
	}
	w := uint16(v) | b8<<8
	if b8 == 0 {
		w |= 1 << 9 // b9 = NOT b8
	}
	return w
}

// Checksum10 computes the 10-bit ST 291-1 Checksum_Word over the supplied 10-bit
// words (DID, SDID, Data_Count, and all User_Data_Words): bits b8..b0 are the
// nine LSBs of the sum of the nine LSBs of each word, and b9 is the inverse of
// b8 (RFC 8331 §2.1).
func Checksum10(words ...uint16) uint16 {
	var sum uint16
	for _, w := range words {
		sum += w & 0x1FF
	}
	cs := sum & 0x1FF
	if cs&0x100 == 0 {
		cs |= 1 << 9 // b9 = NOT b8
	}
	return cs
}

// tenBitWords returns the full list of 10-bit words for the packet in wire order
// (DID, SDID, Data_Count, UDW..., Checksum) with parity and checksum applied.
func (p *Packet) tenBitWords() []uint16 {
	did := EvenParity10(p.DID)
	sdid := EvenParity10(p.SDID)
	dc := EvenParity10(uint8(len(p.UserData)))
	words := make([]uint16, 0, 4+len(p.UserData))
	words = append(words, did, sdid, dc)
	words = append(words, p.UserData...)
	csInputs := make([]uint16, 0, 3+len(p.UserData))
	csInputs = append(csInputs, did, sdid, dc)
	csInputs = append(csInputs, p.UserData...)
	words = append(words, Checksum10(csInputs...))
	return words
}

// marshalSize returns the octet length of one ANC data packet on the wire,
// including the 32-bit location header and word_align padding.
func (p *Packet) marshalSize() int {
	dataBits := 10 * (4 + len(p.UserData)) // DID,SDID,DC,UDW...,CS
	dataBytes := (dataBits + 31) / 32 * 4  // word_align to 32-bit boundary
	return 4 + dataBytes
}

// Marshal serializes the complete ANC RTP payload (header + packets).
func Marshal(h PayloadHeader, packets []Packet) ([]byte, error) {
	if len(packets) > 255 {
		return nil, ErrTooManyANC
	}
	if h.F == 0b01 {
		return nil, ErrInvalidField // RFC 8331 §2.1: 0b01 is not a valid F value
	}
	// ANC data region.
	bw := &bitWriter{}
	for i := range packets {
		p := &packets[i]
		if len(p.UserData) > 255 {
			return nil, ErrDataCount
		}
		if p.LineNumber > 0x7FF || p.HorizontalOffset > 0xFFF || p.StreamNum > 0x7F {
			return nil, ErrFieldRange
		}
		// 32-bit location header.
		var loc uint32
		if p.Color {
			loc |= 1 << 31
		}
		loc |= uint32(p.LineNumber&0x7FF) << 20
		loc |= uint32(p.HorizontalOffset&0xFFF) << 8
		if p.StreamFlag {
			loc |= 1 << 7
		}
		loc |= uint32(p.StreamNum & 0x7F)
		bw.write(loc, 32)
		// 10-bit ST 291-1 words.
		for _, w := range p.tenBitWords() {
			bw.write(uint32(w), 10)
		}
		bw.alignTo(32) // word_align
	}
	data := bw.bytes()

	out := make([]byte, payloadHeaderLen+len(data))
	binary.BigEndian.PutUint16(out[0:2], h.ExtendedSequenceNumber)
	binary.BigEndian.PutUint16(out[2:4], uint16(len(data))) // Length
	out[4] = uint8(len(packets))                            // ANC_Count
	out[5] = h.F << 6                                       // F (top 2 bits), reserved 0
	// out[6], out[7] remain zero (reserved).
	copy(out[payloadHeaderLen:], data)
	return out, nil
}

// Unmarshal parses an ANC RTP payload into its header and ANC data packets.
func Unmarshal(payload []byte) (PayloadHeader, []Packet, error) {
	if len(payload) < payloadHeaderLen {
		return PayloadHeader{}, nil, ErrShortHeader
	}
	h := PayloadHeader{
		ExtendedSequenceNumber: binary.BigEndian.Uint16(payload[0:2]),
		F:                      payload[5] >> 6,
	}
	if h.F == 0b01 {
		return PayloadHeader{}, nil, ErrInvalidField // RFC 8331 §2.1
	}
	length := int(binary.BigEndian.Uint16(payload[2:4]))
	count := int(payload[4])
	data := payload[payloadHeaderLen:]
	if len(data) < length {
		return PayloadHeader{}, nil, ErrShortPayload
	}
	data = data[:length]

	br := &bitReader{buf: data}
	packets := make([]Packet, 0, count)
	for i := 0; i < count; i++ {
		loc, ok := br.read(32)
		if !ok {
			return PayloadHeader{}, nil, ErrShortPayload
		}
		var p Packet
		p.Color = loc&(1<<31) != 0
		p.LineNumber = uint16(loc >> 20 & 0x7FF)
		p.HorizontalOffset = uint16(loc >> 8 & 0xFFF)
		p.StreamFlag = loc&(1<<7) != 0
		p.StreamNum = uint8(loc & 0x7F)

		did, ok1 := br.read(10)
		sdid, ok2 := br.read(10)
		dc, ok3 := br.read(10)
		if !ok1 || !ok2 || !ok3 {
			return PayloadHeader{}, nil, ErrShortPayload
		}
		p.DID = uint8(did)
		p.SDID = uint8(sdid)
		n := int(dc & 0xFF) // actual count is the lower 8 bits
		p.UserData = make([]uint16, n)
		// Accumulate the 10-bit words for the ST 291-1 checksum (DID, SDID,
		// Data_Count, then the User_Data_Words), as transmitted on the wire.
		csInputs := make([]uint16, 0, 3+n)
		csInputs = append(csInputs, uint16(did), uint16(sdid), uint16(dc))
		for j := 0; j < n; j++ {
			w, ok := br.read(10)
			if !ok {
				return PayloadHeader{}, nil, ErrShortPayload
			}
			p.UserData[j] = uint16(w)
			csInputs = append(csInputs, uint16(w))
		}
		cs, ok := br.read(10) // Checksum_Word
		if !ok {
			return PayloadHeader{}, nil, ErrShortPayload
		}
		// Verify the checksum against the carried words (RFC 8331 §2.1, §6.2:
		// the Checksum_Word SHOULD be checked) rather than discarding it.
		if uint16(cs) != Checksum10(csInputs...) {
			return PayloadHeader{}, nil, ErrChecksum
		}
		br.alignTo(32) // skip word_align
		packets = append(packets, p)
	}
	return h, packets, nil
}
