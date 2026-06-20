// Package rtp implements the Real-time Transport Protocol packet format as
// defined in IETF RFC 3550 ("RTP: A Transport Protocol for Real-Time
// Applications"), with the fixed header, CSRC list, and the RFC 3550 §5.3.1
// header-extension mechanism.
//
// RTP is the transport substrate for every SMPTE ST 2110 essence type; ST
// 2110-10 §6 references RFC 3550 directly and ST 2110-20 §6.1.2 reproduces the
// RTP header layout. This package is intentionally free of any SMPTE-specific
// behaviour so it can be reused by the st2110/* packages.
package rtp

import (
	"encoding/binary"
	"errors"
)

// Version is the RTP version implemented by this package (RFC 3550 §5.1).
const Version = 2

// headerBaseLen is the size of the fixed RTP header before CSRC entries and any
// header extension: V/P/X/CC + M/PT + sequence + timestamp + SSRC.
const headerBaseLen = 12

// Errors returned by Unmarshal.
var (
	ErrShortBuffer = errors.New("rtp: buffer too short")
	ErrBadVersion  = errors.New("rtp: unexpected RTP version")
	ErrBadPadding  = errors.New("rtp: invalid padding length")
	ErrTooManyCSRC = errors.New("rtp: more than 15 CSRC identifiers")
	ErrExtNotMul4  = errors.New("rtp: header extension data not a multiple of 4 octets")
)

// Header is the RTP fixed header (RFC 3550 §5.1).
//
// The bit layout, most significant bit first, is:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|V=2|P|X|  CC   |M|     PT      |       sequence number         |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                           timestamp                           |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|           synchronization source (SSRC) identifier            |
//	+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
//	|            contributing source (CSRC) identifiers             |
//	|                             ....                              |
//	+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
type Header struct {
	// Version is the RTP version (2 per RFC 3550). Marshal writes this value as
	// given; a zero value is treated as Version (2) for convenience.
	Version uint8
	// Padding is the P bit. When set, the packet payload ends with one or more
	// padding octets whose final octet is the padding count (handled by Packet).
	Padding bool
	// Extension is the X bit. When set, a single RFC 3550 §5.3.1 header
	// extension follows the CSRC list.
	Extension bool
	// Marker is the M bit. Its meaning is defined by the RTP profile / payload
	// format (e.g. ST 2110-20 §6.1.2: last packet of a frame or field).
	Marker bool
	// PayloadType is the 7-bit PT field.
	PayloadType uint8
	// SequenceNumber is the 16-bit RTP sequence number.
	SequenceNumber uint16
	// Timestamp is the 32-bit RTP timestamp.
	Timestamp uint32
	// SSRC is the synchronization source identifier.
	SSRC uint32
	// CSRC holds 0..15 contributing source identifiers.
	CSRC []uint32

	// ExtensionProfile is the 16-bit "defined by profile" field of the header
	// extension (e.g. 0xBEDE for the RFC 8285 one-byte form). Only meaningful
	// when Extension is true.
	ExtensionProfile uint16
	// ExtensionData is the header-extension payload. Its length must be a
	// multiple of 4 octets. Only meaningful when Extension is true.
	ExtensionData []byte
}

// version returns the version to emit, defaulting a zero value to 2.
func (h *Header) version() uint8 {
	if h.Version == 0 {
		return Version
	}
	return h.Version
}

// MarshalSize returns the number of octets Header.MarshalTo will write.
func (h *Header) MarshalSize() int {
	n := headerBaseLen + 4*len(h.CSRC)
	if h.Extension {
		n += 4 + len(h.ExtensionData)
	}
	return n
}

// Marshal returns the wire encoding of the header.
func (h *Header) Marshal() ([]byte, error) {
	buf := make([]byte, h.MarshalSize())
	if _, err := h.MarshalTo(buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// MarshalTo writes the header into buf and returns the number of octets written.
func (h *Header) MarshalTo(buf []byte) (int, error) {
	if len(h.CSRC) > 15 {
		return 0, ErrTooManyCSRC
	}
	if h.Extension && len(h.ExtensionData)%4 != 0 {
		return 0, ErrExtNotMul4
	}
	size := h.MarshalSize()
	if len(buf) < size {
		return 0, ErrShortBuffer
	}

	b0 := h.version()<<6 | uint8(len(h.CSRC))&0x0f
	if h.Padding {
		b0 |= 1 << 5
	}
	if h.Extension {
		b0 |= 1 << 4
	}
	buf[0] = b0

	b1 := h.PayloadType & 0x7f
	if h.Marker {
		b1 |= 1 << 7
	}
	buf[1] = b1

	binary.BigEndian.PutUint16(buf[2:4], h.SequenceNumber)
	binary.BigEndian.PutUint32(buf[4:8], h.Timestamp)
	binary.BigEndian.PutUint32(buf[8:12], h.SSRC)

	off := headerBaseLen
	for _, c := range h.CSRC {
		binary.BigEndian.PutUint32(buf[off:off+4], c)
		off += 4
	}

	if h.Extension {
		binary.BigEndian.PutUint16(buf[off:off+2], h.ExtensionProfile)
		binary.BigEndian.PutUint16(buf[off+2:off+4], uint16(len(h.ExtensionData)/4))
		off += 4
		off += copy(buf[off:], h.ExtensionData)
	}
	return off, nil
}

// Unmarshal parses an RTP header from the front of buf, returning the number of
// octets consumed (the header length). It does not copy slices; ExtensionData
// aliases buf.
func (h *Header) Unmarshal(buf []byte) (int, error) {
	if len(buf) < headerBaseLen {
		return 0, ErrShortBuffer
	}
	h.Version = buf[0] >> 6
	if h.Version != Version {
		return 0, ErrBadVersion
	}
	h.Padding = buf[0]>>5&1 == 1
	h.Extension = buf[0]>>4&1 == 1
	cc := int(buf[0] & 0x0f)

	h.Marker = buf[1]>>7&1 == 1
	h.PayloadType = buf[1] & 0x7f
	h.SequenceNumber = binary.BigEndian.Uint16(buf[2:4])
	h.Timestamp = binary.BigEndian.Uint32(buf[4:8])
	h.SSRC = binary.BigEndian.Uint32(buf[8:12])

	off := headerBaseLen
	if len(buf) < off+4*cc {
		return 0, ErrShortBuffer
	}
	if cc > 0 {
		h.CSRC = make([]uint32, cc)
		for i := 0; i < cc; i++ {
			h.CSRC[i] = binary.BigEndian.Uint32(buf[off : off+4])
			off += 4
		}
	} else {
		h.CSRC = nil
	}

	if h.Extension {
		if len(buf) < off+4 {
			return 0, ErrShortBuffer
		}
		h.ExtensionProfile = binary.BigEndian.Uint16(buf[off : off+2])
		words := int(binary.BigEndian.Uint16(buf[off+2 : off+4]))
		off += 4
		extLen := words * 4
		if len(buf) < off+extLen {
			return 0, ErrShortBuffer
		}
		h.ExtensionData = buf[off : off+extLen]
		off += extLen
	} else {
		h.ExtensionProfile = 0
		h.ExtensionData = nil
	}
	return off, nil
}

// Packet is a complete RTP packet: a Header, its payload, and (when
// Header.Padding is set) a count of trailing padding octets.
type Packet struct {
	Header  Header
	Payload []byte
	// PaddingSize is the number of padding octets appended after Payload,
	// including the final length octet itself (RFC 3550 §5.1). It must be at
	// least 1 when Header.Padding is set.
	PaddingSize uint8
}

// MarshalSize returns the number of octets Packet.MarshalTo will write.
func (p *Packet) MarshalSize() int {
	n := p.Header.MarshalSize() + len(p.Payload)
	if p.Header.Padding {
		n += int(p.PaddingSize)
	}
	return n
}

// Marshal returns the wire encoding of the packet.
func (p *Packet) Marshal() ([]byte, error) {
	buf := make([]byte, p.MarshalSize())
	if _, err := p.MarshalTo(buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// MarshalTo writes the packet into buf and returns the octets written.
func (p *Packet) MarshalTo(buf []byte) (int, error) {
	if p.Header.Padding && p.PaddingSize == 0 {
		return 0, ErrBadPadding
	}
	if len(buf) < p.MarshalSize() {
		return 0, ErrShortBuffer
	}
	n, err := p.Header.MarshalTo(buf)
	if err != nil {
		return 0, err
	}
	n += copy(buf[n:], p.Payload)
	if p.Header.Padding {
		// Zero the padding region and write the count in the final octet.
		pad := int(p.PaddingSize)
		for i := 0; i < pad-1; i++ {
			buf[n+i] = 0
		}
		buf[n+pad-1] = p.PaddingSize
		n += pad
	}
	return n, nil
}

// Unmarshal parses a complete RTP packet from buf. Payload (and any header
// extension) aliases buf; copy if buf will be reused.
func (p *Packet) Unmarshal(buf []byte) error {
	hlen, err := p.Header.Unmarshal(buf)
	if err != nil {
		return err
	}
	rest := buf[hlen:]
	if p.Header.Padding {
		if len(rest) == 0 {
			return ErrBadPadding
		}
		pad := int(rest[len(rest)-1])
		if pad == 0 || pad > len(rest) {
			return ErrBadPadding
		}
		p.PaddingSize = uint8(pad)
		p.Payload = rest[:len(rest)-pad]
	} else {
		p.PaddingSize = 0
		p.Payload = rest
	}
	return nil
}
