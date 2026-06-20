// Package ttml implements SMPTE ST 2110-43 "Timed Text", the carriage of TTML2
// documents over RTP using the payload format of IETF RFC 8759. ST 2110-43
// requires a 90 kHz RTP clock (§4.2) and IMSC1.2 TTML2 documents; RFC 8759
// defines the wire format: a 16-bit Reserved field, a 16-bit Length, and the
// User Data Words (the UTF-8 TTML document, possibly fragmented across packets).
package ttml

import (
	"encoding/binary"
	"errors"
	"unicode/utf8"

	"github.com/szatmary/go2110/rtp"
)

// RTPClockRate is the ST 2110-43 RTP clock rate (§4.2): 90 kHz.
const RTPClockRate = 90000

// payloadHeaderLen is the RFC 8759 payload header size: Reserved(2) + Length(2).
const payloadHeaderLen = 4

// Errors for TTML payload handling.
var (
	ErrShortPayload = errors.New("st2110/ttml: payload shorter than the RFC 8759 header")
	ErrBadLength    = errors.New("st2110/ttml: Length field exceeds payload")
	// ErrTimestampMismatch is returned when packets that are supposed to belong
	// to one document do not share the RTP timestamp (RFC 8759 §4.2: "Documents
	// spread across multiple packets MUST use the same timestamp").
	ErrTimestampMismatch = errors.New("st2110/ttml: packets of one document must share the RTP timestamp")
	// ErrSequenceGap is returned when a document's packets are not consecutive in
	// sequence number (RFC 8759 §4.2/§8: "different consecutive Sequence Numbers").
	ErrSequenceGap = errors.New("st2110/ttml: document packets must have consecutive sequence numbers")
	// ErrEmptyDocument is returned for a zero-length (empty) document, which is
	// invalid and MUST be discarded (RFC 8759 §6).
	ErrEmptyDocument = errors.New("st2110/ttml: empty (zero-length) document must be discarded")
)

// MarshalPacket builds an RFC 8759 RTP payload carrying the given User Data Words
// (a whole document or one fragment). The Reserved field is set to 0 (§Figure 1).
func MarshalPacket(udw []byte) []byte {
	out := make([]byte, payloadHeaderLen+len(udw))
	// out[0:2] Reserved = 0.
	binary.BigEndian.PutUint16(out[2:4], uint16(len(udw)))
	copy(out[payloadHeaderLen:], udw)
	return out
}

// ParsePacket extracts the User Data Words from an RFC 8759 RTP payload.
func ParsePacket(payload []byte) ([]byte, error) {
	if len(payload) < payloadHeaderLen {
		return nil, ErrShortPayload
	}
	length := int(binary.BigEndian.Uint16(payload[2:4]))
	if payloadHeaderLen+length > len(payload) {
		return nil, ErrBadLength
	}
	return payload[payloadHeaderLen : payloadHeaderLen+length], nil
}

// PacketizeOptions controls TTML RTP packetization.
type PacketizeOptions struct {
	PayloadType uint8
	SSRC        uint32
	// Timestamp is the RTP timestamp marking the epoch of the document; all
	// fragments of one document share it (RFC 8759 §8).
	Timestamp uint32
	// StartSequence is the RTP sequence number of the first fragment.
	StartSequence uint16
	// MaxUDW is the maximum User Data Words per packet (octets). If zero, a
	// value based on the Standard UDP Size Limit (1460 − 12 RTP − 4 header) is
	// used.
	MaxUDW int
}

// defaultMaxUDW is the User Data Words budget for the Standard UDP Size Limit.
const defaultMaxUDW = 1460 - 12 - payloadHeaderLen

// Packetize splits a single TTML document into RFC 8759 RTP packets. Fragments
// split only at UTF-8 character boundaries (§8) and share one RTP timestamp; the
// marker bit is set on the packet carrying the last fragment (§4.1).
func Packetize(document []byte, opts PacketizeOptions) []rtp.Packet {
	maxUDW := opts.MaxUDW
	if maxUDW <= 0 {
		maxUDW = defaultMaxUDW
	}
	var packets []rtp.Packet
	seq := opts.StartSequence
	for off := 0; off < len(document) || len(packets) == 0; {
		remain := document[off:]
		take := len(remain)
		if take > maxUDW {
			take = utf8SafeCut(remain, maxUDW)
		}
		off += take
		packets = append(packets, rtp.Packet{
			Header: rtp.Header{
				Version:        rtp.Version,
				PayloadType:    opts.PayloadType,
				SequenceNumber: seq,
				Timestamp:      opts.Timestamp,
				SSRC:           opts.SSRC,
				Marker:         off >= len(document), // last fragment
			},
			Payload: MarshalPacket(remain[:take]),
		})
		seq++
		if take == 0 { // empty document: a single zero-length packet
			break
		}
	}
	return packets
}

// utf8SafeCut returns the largest cut length ≤ max that does not split a UTF-8
// multibyte sequence. If a single rune exceeds max (pathological), it returns max
// to guarantee progress.
func utf8SafeCut(b []byte, max int) int {
	cut := max
	for cut > 0 && !utf8.RuneStart(b[cut]) {
		cut--
	}
	if cut == 0 {
		return max
	}
	return cut
}

// KeepAlive returns a keep-alive packet: a zero-length payload with the marker
// bit set, as permitted between documents by ST 2110-43 §4.3.
func KeepAlive(opts PacketizeOptions) rtp.Packet {
	return rtp.Packet{
		Header: rtp.Header{
			Version:        rtp.Version,
			PayloadType:    opts.PayloadType,
			SequenceNumber: opts.StartSequence,
			Timestamp:      opts.Timestamp,
			SSRC:           opts.SSRC,
			Marker:         true,
		},
		Payload: MarshalPacket(nil),
	}
}

// Depacketize reassembles one TTML document from the RTP packets of a single
// document, concatenating their User Data Words up to and including the packet
// whose marker bit is set (RFC 8759 §8). It enforces the document framing rules:
// every contributing packet must share one RTP timestamp (§4.2) and carry
// consecutive ascending sequence numbers (§4.2/§8); a reassembled document of
// zero length is invalid and is rejected (§6, "empty documents ... MUST be
// discarded"). Packets after the marker (a new document) are not consumed.
func Depacketize(packets []rtp.Packet) ([]byte, error) {
	if len(packets) == 0 {
		return nil, ErrEmptyDocument
	}
	ts := packets[0].Header.Timestamp
	firstSeq := packets[0].Header.SequenceNumber
	var doc []byte
	for i := range packets {
		if packets[i].Header.Timestamp != ts {
			return nil, ErrTimestampMismatch
		}
		if packets[i].Header.SequenceNumber != firstSeq+uint16(i) {
			return nil, ErrSequenceGap
		}
		udw, err := ParsePacket(packets[i].Payload)
		if err != nil {
			return nil, err
		}
		doc = append(doc, udw...)
		if packets[i].Header.Marker {
			break
		}
	}
	if len(doc) == 0 {
		return nil, ErrEmptyDocument
	}
	return doc, nil
}
