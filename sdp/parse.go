package sdp

import (
	"errors"
	"strconv"
	"strings"
)

// ErrInvalidSDP is returned for structurally invalid SDP input.
var ErrInvalidSDP = errors.New("sdp: invalid SDP")

// Unmarshal parses an SDP object from text. It accepts CRLF or bare LF line
// endings and ignores blank lines, which makes it tolerant of the line-wrapped
// examples found in the standards. Unknown lines are preserved where they map to
// a modeled field (attributes), and otherwise skipped.
func Unmarshal(text string) (*SessionDescription, error) {
	s := &SessionDescription{}
	var cur *MediaDescription // nil while in the session section

	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		typ, value, ok := strings.Cut(line, "=")
		if !ok || len(typ) != 1 {
			return nil, ErrInvalidSDP
		}
		// SDP forbids whitespace around the field type but the standards' PDFs
		// often indent continuation; trim a leading space on the value only.
		value = strings.TrimLeft(value, " \t")

		switch typ {
		case "v":
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, ErrInvalidSDP
			}
			s.Version = n
		case "o":
			o, err := parseOrigin(value)
			if err != nil {
				return nil, err
			}
			s.Origin = o
		case "s":
			s.SessionName = value
		case "i":
			if cur != nil {
				cur.Information = value
			} else {
				s.Information = value
			}
		case "u":
			s.URI = value
		case "e":
			s.Email = append(s.Email, value)
		case "p":
			s.Phone = append(s.Phone, value)
		case "c":
			c, err := parseConnection(value)
			if err != nil {
				return nil, err
			}
			if cur != nil {
				cur.Connection = c
			} else {
				s.Connection = c
			}
		case "b":
			bw, err := parseBandwidth(value)
			if err != nil {
				return nil, err
			}
			if cur != nil {
				cur.Bandwidth = append(cur.Bandwidth, bw)
			} else {
				s.Bandwidth = append(s.Bandwidth, bw)
			}
		case "t":
			t, err := parseTiming(value)
			if err != nil {
				return nil, err
			}
			s.Timing = append(s.Timing, t)
		case "a":
			attr := parseAttribute(value)
			if cur != nil {
				cur.Attributes = append(cur.Attributes, attr)
			} else {
				s.Attributes = append(s.Attributes, attr)
			}
		case "m":
			m, err := parseMedia(value)
			if err != nil {
				return nil, err
			}
			s.MediaDescriptions = append(s.MediaDescriptions, m)
			cur = m
		case "r", "z", "k":
			// Repeat/zone/key lines are not modeled; ignore.
		default:
			// Unknown line type; ignore for forward compatibility.
		}
	}
	if s.SessionName == "" && s.Origin == (Origin{}) {
		return nil, ErrInvalidSDP
	}
	return s, nil
}

func parseOrigin(v string) (Origin, error) {
	f := strings.Fields(v)
	if len(f) != 6 {
		return Origin{}, ErrInvalidSDP
	}
	return Origin{
		Username: f[0], SessionID: f[1], SessionVersion: f[2],
		NetworkType: f[3], AddressType: f[4], UnicastAddress: f[5],
	}, nil
}

func parseConnection(v string) (*ConnectionData, error) {
	f := strings.Fields(v)
	if len(f) != 3 {
		return nil, ErrInvalidSDP
	}
	return &ConnectionData{NetworkType: f[0], AddressType: f[1], Address: f[2]}, nil
}

func parseBandwidth(v string) (Bandwidth, error) {
	typ, val, ok := strings.Cut(v, ":")
	if !ok {
		return Bandwidth{}, ErrInvalidSDP
	}
	n, err := strconv.ParseUint(strings.TrimSpace(val), 10, 64)
	if err != nil {
		return Bandwidth{}, ErrInvalidSDP
	}
	return Bandwidth{Type: typ, Value: n}, nil
}

func parseTiming(v string) (Timing, error) {
	f := strings.Fields(v)
	if len(f) != 2 {
		return Timing{}, ErrInvalidSDP
	}
	start, err1 := strconv.ParseUint(f[0], 10, 64)
	stop, err2 := strconv.ParseUint(f[1], 10, 64)
	if err1 != nil || err2 != nil {
		return Timing{}, ErrInvalidSDP
	}
	return Timing{Start: start, Stop: stop}, nil
}

func parseAttribute(v string) Attribute {
	key, val, ok := strings.Cut(v, ":")
	if !ok {
		return Attribute{Key: v}
	}
	return Attribute{Key: key, Value: val, HasValue: true}
}

func parseMedia(v string) (*MediaDescription, error) {
	f := strings.Fields(v)
	if len(f) < 3 {
		return nil, ErrInvalidSDP
	}
	m := &MediaDescription{Media: f[0], Protocol: f[2]}
	portStr, countStr, hasCount := strings.Cut(f[1], "/")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, ErrInvalidSDP
	}
	m.Port = port
	if hasCount {
		c, err := strconv.Atoi(countStr)
		if err != nil {
			return nil, ErrInvalidSDP
		}
		m.PortCount = c
	}
	if len(f) > 3 {
		m.Formats = f[3:]
	}
	return m, nil
}
