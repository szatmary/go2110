package rtp

import "errors"

// RFC 8285 ("A General Mechanism for RTP Header Extensions") defines two
// concrete encodings carried inside the RFC 3550 §5.3.1 header extension:
// a one-byte form (profile 0xBEDE) and a two-byte form (profile 0x100x). ST
// 2110-10 §6 and ST 2110-20 §6.1.2 permit RFC 8285 header extensions, so these
// helpers parse and build the element list that lives in Header.ExtensionData.

// Profile identifiers for the RFC 8285 header extension forms.
const (
	// ExtProfileOneByte is the "defined by profile" value for the one-byte
	// header extension form (RFC 8285 §4.2).
	ExtProfileOneByte uint16 = 0xBEDE
	// ExtProfileTwoByteBase is the high 12 bits of the two-byte form profile;
	// the low 4 bits (appbits) are application defined (RFC 8285 §4.3).
	ExtProfileTwoByteBase uint16 = 0x1000
	extProfileTwoByteMask uint16 = 0xfff0
)

// Errors for RFC 8285 extension element parsing.
var (
	ErrBadExtElement = errors.New("rtp: malformed RFC 8285 extension element")
	ErrBadExtID      = errors.New("rtp: RFC 8285 extension element ID out of range")
)

// ExtensionElement is a single RFC 8285 header-extension element: a local
// identifier and its value bytes.
type ExtensionElement struct {
	ID    uint8
	Value []byte
}

// IsTwoByteProfile reports whether profile selects the RFC 8285 two-byte form.
func IsTwoByteProfile(profile uint16) bool {
	return profile&extProfileTwoByteMask == ExtProfileTwoByteBase
}

// ParseExtensionElements decodes the RFC 8285 element list in data according to
// profile (ExtProfileOneByte or a two-byte profile). Padding octets (ID 0) are
// skipped. ID 15 in the one-byte form terminates parsing per RFC 8285 §4.2.
func ParseExtensionElements(profile uint16, data []byte) ([]ExtensionElement, error) {
	if IsTwoByteProfile(profile) {
		return parseTwoByte(data)
	}
	if profile == ExtProfileOneByte {
		return parseOneByte(data)
	}
	return nil, ErrBadExtElement
}

func parseOneByte(data []byte) ([]ExtensionElement, error) {
	var out []ExtensionElement
	for i := 0; i < len(data); {
		b := data[i]
		if b == 0 { // padding
			i++
			continue
		}
		id := b >> 4
		length := int(b&0x0f) + 1 // encoded length is len-1
		if id == 15 {             // reserved: stop processing
			break
		}
		i++
		if i+length > len(data) {
			return nil, ErrBadExtElement
		}
		out = append(out, ExtensionElement{ID: id, Value: data[i : i+length]})
		i += length
	}
	return out, nil
}

func parseTwoByte(data []byte) ([]ExtensionElement, error) {
	var out []ExtensionElement
	for i := 0; i < len(data); {
		id := data[i]
		if id == 0 { // padding
			i++
			continue
		}
		if i+1 >= len(data) {
			return nil, ErrBadExtElement
		}
		length := int(data[i+1])
		i += 2
		if i+length > len(data) {
			return nil, ErrBadExtElement
		}
		out = append(out, ExtensionElement{ID: id, Value: data[i : i+length]})
		i += length
	}
	return out, nil
}

// BuildExtensionElements encodes elems into a header-extension payload using the
// form selected by profile, padding the result to a multiple of 4 octets. The
// returned bytes are suitable for Header.ExtensionData with Header.Extension set
// and Header.ExtensionProfile = profile.
func BuildExtensionElements(profile uint16, elems []ExtensionElement) ([]byte, error) {
	twoByte := IsTwoByteProfile(profile)
	var out []byte
	for _, e := range elems {
		if twoByte {
			if e.ID == 0 {
				return nil, ErrBadExtID
			}
			if len(e.Value) > 255 {
				return nil, ErrBadExtElement
			}
			out = append(out, e.ID, byte(len(e.Value)))
			out = append(out, e.Value...)
		} else {
			if e.ID == 0 || e.ID > 14 {
				return nil, ErrBadExtID
			}
			if len(e.Value) < 1 || len(e.Value) > 16 {
				return nil, ErrBadExtElement
			}
			out = append(out, e.ID<<4|byte(len(e.Value)-1))
			out = append(out, e.Value...)
		}
	}
	for len(out)%4 != 0 { // pad with zero octets
		out = append(out, 0)
	}
	return out, nil
}
