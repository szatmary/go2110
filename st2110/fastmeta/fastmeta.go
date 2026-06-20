// Package fastmeta implements SMPTE ST 2110-41 "Fast Metadata Framework": a
// flexible RTP payload carrying a sequence of Data Item Packages, each tagged
// with a 22-bit Data Item Type, a K-bit, and a 9-bit length in 32-bit words
// (§5.4). The contents of each Data Item are defined by the document for its
// Data Item Type and are opaque to this framework.
package fastmeta

import (
	"encoding/binary"
	"errors"
	"math"
)

// diHeaderLen is the size of a Data Item Package header (§5.4): Type(22) +
// K(1) + Length(9) = 32 bits.
const diHeaderLen = 4

// MaxDataItemWords is the largest Data Item Length (9-bit field): 511 32-bit
// content words.
const MaxDataItemWords = 511

// Data Item Type ranges (§8).
const (
	TypeMaskSMPTE     = 0x000000 // 0x000000–0x0FFFFF SMPTE
	TypeExperimentLow = 0x3FF000 // 0x3FF000–0x3FFFFF experimental
)

// Errors for Data Item marshaling.
var (
	ErrContentsAlign = errors.New("st2110/fastmeta: Data Item Contents not a multiple of 32 bits")
	ErrContentsRange = errors.New("st2110/fastmeta: Data Item length must be 1..511 content words")
	ErrTypeRange     = errors.New("st2110/fastmeta: Data Item Type exceeds 22 bits")
	ErrShortPayload  = errors.New("st2110/fastmeta: payload shorter than declared Data Item")
)

// DataItem is one Data Item Package (§5.4). Contents is the Data Item Contents,
// which must be a non-empty multiple of 4 octets (1..511 32-bit words).
type DataItem struct {
	// Type is the 22-bit Data Item Type identifying the contents.
	Type uint32
	// K is the Data Item K-bit; its meaning is defined per Data Item Type
	// (or by the Annex A Simple Object Segmentation Method).
	K bool
	// Contents is the Data Item Contents (multiple of 4 octets).
	Contents []byte
}

// MarshalPayload serializes a sequence of Data Item Packages into an RTP payload
// (§5.4): a contiguous run of packages with no inter-package padding. An empty
// slice yields an empty payload (a valid zero-item keep-alive packet, §5.1).
func MarshalPayload(items []DataItem) ([]byte, error) {
	total := 0
	for _, it := range items {
		if len(it.Contents)%4 != 0 {
			return nil, ErrContentsAlign
		}
		words := len(it.Contents) / 4
		if words < 1 || words > MaxDataItemWords {
			return nil, ErrContentsRange
		}
		if it.Type > 0x3FFFFF {
			return nil, ErrTypeRange
		}
		total += diHeaderLen + len(it.Contents)
	}
	out := make([]byte, 0, total)
	var hdr [4]byte
	for _, it := range items {
		words := uint32(len(it.Contents) / 4)
		h := it.Type<<10 | words&0x1FF
		if it.K {
			h |= 1 << 9
		}
		binary.BigEndian.PutUint32(hdr[:], h)
		out = append(out, hdr[:]...)
		out = append(out, it.Contents...)
	}
	return out, nil
}

// UnmarshalPayload parses an RTP payload into its Data Item Packages.
func UnmarshalPayload(payload []byte) ([]DataItem, error) {
	var items []DataItem
	for off := 0; off < len(payload); {
		if off+diHeaderLen > len(payload) {
			return nil, ErrShortPayload
		}
		h := binary.BigEndian.Uint32(payload[off : off+4])
		off += diHeaderLen
		words := int(h & 0x1FF)
		if words == 0 {
			return nil, ErrContentsRange
		}
		n := words * 4
		if off+n > len(payload) {
			return nil, ErrShortPayload
		}
		items = append(items, DataItem{
			Type:     h >> 10 & 0x3FFFFF,
			K:        h>>9&1 == 1,
			Contents: payload[off : off+n],
		})
		off += n
	}
	return items, nil
}

// Beta is the scaling factor β of the ST 2110-41 Network Compatibility Model (§7).
const Beta = 1.1

// TDrain returns TDRAIN in seconds for the Network Compatibility Model (§7):
// 1 / MAX(800, RNOMINAL × β), where rNominal is the long-term packet rate.
func TDrain(rNominal float64) float64 {
	return 1.0 / math.Max(800, rNominal*Beta)
}

// CMax returns the CMAX value (packets) for the Network Compatibility Model (§7):
// MAX(4, INT(RNOMINAL / 43200)).
func CMax(rNominal float64) int {
	v := int(rNominal / 43200)
	if v < 4 {
		return 4
	}
	return v
}
