// Package sdp implements the Session Description Protocol (IETF RFC 4566) with
// the attributes used to describe SMPTE ST 2110 streams: the RFC 7273 timestamp
// reference clock (ts-refclk) and media clock (mediaclk) signalling required by
// ST 2110-10 §8, RFC 4570 source filters, and RFC 5888/7104 DUP grouping.
//
// The core types model the SDP grammar faithfully (field order, CRLF line
// endings); typed helpers in attributes.go build and read the 2110-specific
// attributes on top.
package sdp

import (
	"strconv"
	"strings"
)

// crlf is the SDP line terminator (RFC 4566 §5: each line ends with CRLF).
const crlf = "\r\n"

// SessionDescription is a complete SDP object (RFC 4566 §5).
type SessionDescription struct {
	// Version is the SDP protocol version (v=), always 0.
	Version int
	// Origin is the o= line.
	Origin Origin
	// SessionName is the s= line. RFC 4566 requires it be non-empty; use " ".
	SessionName string
	// Information is the optional session-level i= line.
	Information string
	// URI is the optional u= line.
	URI string
	// Email holds optional e= lines.
	Email []string
	// Phone holds optional p= lines.
	Phone []string
	// Connection is the optional session-level c= line.
	Connection *ConnectionData
	// Bandwidth holds optional session-level b= lines.
	Bandwidth []Bandwidth
	// Timing holds one or more t= lines (RFC 4566 requires at least one).
	Timing []Timing
	// Attributes holds session-level a= lines in order.
	Attributes []Attribute
	// MediaDescriptions holds the m= sections in order.
	MediaDescriptions []*MediaDescription
}

// Origin is the o= line: username, session id/version, and the originating
// network address (RFC 4566 §5.2).
type Origin struct {
	Username       string
	SessionID      string
	SessionVersion string
	NetworkType    string // typically "IN"
	AddressType    string // "IP4" or "IP6"
	UnicastAddress string
}

func (o Origin) String() string {
	return strings.Join([]string{
		o.Username, o.SessionID, o.SessionVersion,
		o.NetworkType, o.AddressType, o.UnicastAddress,
	}, " ")
}

// ConnectionData is a c= line (RFC 4566 §5.7). Address may carry the multicast
// "/TTL" and "/count" suffixes verbatim (e.g. "239.100.9.10/32").
type ConnectionData struct {
	NetworkType string // "IN"
	AddressType string // "IP4"/"IP6"
	Address     string
}

func (c ConnectionData) String() string {
	return c.NetworkType + " " + c.AddressType + " " + c.Address
}

// Bandwidth is a b= line (RFC 4566 §5.8): <bwtype>:<bandwidth>.
type Bandwidth struct {
	Type  string
	Value uint64
}

// Timing is a t= line (RFC 4566 §5.9): start and stop times in NTP seconds, or
// 0 0 for an unbounded session.
type Timing struct {
	Start uint64
	Stop  uint64
}

// Attribute is an a= line. Value is empty for a property (value-less) attribute
// such as "a=recvonly"; HasValue distinguishes "a=key:" from "a=key".
type Attribute struct {
	Key      string
	Value    string
	HasValue bool
}

// MediaDescription is an m= section (RFC 4566 §5.14) and its subordinate lines.
type MediaDescription struct {
	// Media is the media type: "video", "audio", "application", etc.
	Media string
	// Port is the transport port from the m= line.
	Port int
	// PortCount is the optional "/count" on the m= line (0 means absent).
	PortCount int
	// Protocol is the transport protocol, e.g. "RTP/AVP".
	Protocol string
	// Formats are the media format descriptors (payload types for RTP/AVP).
	Formats []string
	// Information is the optional media-level i= line.
	Information string
	// Connection is the optional media-level c= line.
	Connection *ConnectionData
	// Bandwidth holds optional media-level b= lines.
	Bandwidth []Bandwidth
	// Attributes holds media-level a= lines in order.
	Attributes []Attribute
}

// Marshal returns the SDP object encoded per RFC 4566, with CRLF line endings
// and fields in the order mandated by the grammar.
func (s *SessionDescription) Marshal() string {
	var b strings.Builder
	writeLine(&b, "v", strconv.Itoa(s.Version))
	writeLine(&b, "o", s.Origin.String())
	writeLine(&b, "s", s.SessionName)
	if s.Information != "" {
		writeLine(&b, "i", s.Information)
	}
	if s.URI != "" {
		writeLine(&b, "u", s.URI)
	}
	for _, e := range s.Email {
		writeLine(&b, "e", e)
	}
	for _, p := range s.Phone {
		writeLine(&b, "p", p)
	}
	if s.Connection != nil {
		writeLine(&b, "c", s.Connection.String())
	}
	for _, bw := range s.Bandwidth {
		writeLine(&b, "b", bw.Type+":"+strconv.FormatUint(bw.Value, 10))
	}
	for _, t := range s.Timing {
		writeLine(&b, "t", strconv.FormatUint(t.Start, 10)+" "+strconv.FormatUint(t.Stop, 10))
	}
	for _, a := range s.Attributes {
		writeLine(&b, "a", a.line())
	}
	for _, m := range s.MediaDescriptions {
		m.marshalInto(&b)
	}
	return b.String()
}

func (m *MediaDescription) marshalInto(b *strings.Builder) {
	port := strconv.Itoa(m.Port)
	if m.PortCount > 0 {
		port += "/" + strconv.Itoa(m.PortCount)
	}
	fields := m.Media + " " + port + " " + m.Protocol
	if len(m.Formats) > 0 {
		fields += " " + strings.Join(m.Formats, " ")
	}
	writeLine(b, "m", fields)
	if m.Information != "" {
		writeLine(b, "i", m.Information)
	}
	if m.Connection != nil {
		writeLine(b, "c", m.Connection.String())
	}
	for _, bw := range m.Bandwidth {
		writeLine(b, "b", bw.Type+":"+strconv.FormatUint(bw.Value, 10))
	}
	for _, a := range m.Attributes {
		writeLine(b, "a", a.line())
	}
}

// line renders the attribute value portion (after "a=").
func (a Attribute) line() string {
	if a.HasValue {
		return a.Key + ":" + a.Value
	}
	return a.Key
}

func writeLine(b *strings.Builder, typ, value string) {
	b.WriteString(typ)
	b.WriteByte('=')
	b.WriteString(value)
	b.WriteString(crlf)
}

// attrValue returns the value of the first attribute with the given key and
// whether it was present (with a value).
func attrValue(attrs []Attribute, key string) (string, bool) {
	for _, a := range attrs {
		if a.Key == key && a.HasValue {
			return a.Value, true
		}
	}
	return "", false
}

// hasFlag reports whether a value-less attribute with the given key is present.
func hasFlag(attrs []Attribute, key string) bool {
	for _, a := range attrs {
		if a.Key == key && !a.HasValue {
			return true
		}
	}
	return false
}

// GetAttribute returns the value of the first session-level attribute with key.
func (s *SessionDescription) GetAttribute(key string) (string, bool) {
	return attrValue(s.Attributes, key)
}

// GetAttribute returns the value of the first attribute on the media with key.
func (m *MediaDescription) GetAttribute(key string) (string, bool) {
	return attrValue(m.Attributes, key)
}
