package ttml

import (
	"testing"

	"github.com/szatmary/go2110/rtp"
)

func pkt(seq uint16, ts uint32, marker bool, udw []byte) rtp.Packet {
	return rtp.Packet{
		Header:  rtp.Header{Version: rtp.Version, SequenceNumber: seq, Timestamp: ts, Marker: marker},
		Payload: MarshalPacket(udw),
	}
}

// TestT2_DepacketizeValidation pins the RFC 8759 §4.2/§6/§8 reassembly rules:
// shared timestamp, consecutive sequence numbers, and discarding empty documents.
func TestT2_DepacketizeValidation(t *testing.T) {
	// Shared timestamp + consecutive sequence: valid.
	good := []rtp.Packet{
		pkt(10, 900, false, []byte("<tt>")),
		pkt(11, 900, true, []byte("</tt>")),
	}
	if doc, err := Depacketize(good); err != nil || string(doc) != "<tt></tt>" {
		t.Fatalf("valid reassembly = %q err=%v", doc, err)
	}

	// Mismatched timestamp within a document: rejected (§4.2).
	tsBad := []rtp.Packet{
		pkt(10, 900, false, []byte("<tt>")),
		pkt(11, 901, true, []byte("</tt>")),
	}
	if _, err := Depacketize(tsBad); err != ErrTimestampMismatch {
		t.Fatalf("timestamp mismatch = %v, want ErrTimestampMismatch", err)
	}

	// Non-consecutive sequence numbers: rejected (§8).
	seqBad := []rtp.Packet{
		pkt(10, 900, false, []byte("<tt>")),
		pkt(12, 900, true, []byte("</tt>")),
	}
	if _, err := Depacketize(seqBad); err != ErrSequenceGap {
		t.Fatalf("sequence gap = %v, want ErrSequenceGap", err)
	}

	// Zero-length (empty) document: discarded (§6).
	empty := []rtp.Packet{pkt(10, 900, true, nil)}
	if _, err := Depacketize(empty); err != ErrEmptyDocument {
		t.Fatalf("empty document = %v, want ErrEmptyDocument", err)
	}
}
