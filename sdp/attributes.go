package sdp

import (
	"errors"
	"strconv"
	"strings"
)

// ErrBadAttribute is returned when a typed attribute value is malformed.
var ErrBadAttribute = errors.New("sdp: malformed attribute value")

// ErrBadEUI is returned when an EUI-48 (localmac) or EUI-64 (PTP grandmaster
// clockIdentity) value is not N hyphen-separated 2-hex-digit octets (RFC 7273
// Figure 1: EUI64 = 7(2HEXDIG "-") 2HEXDIG).
var ErrBadEUI = errors.New("sdp: malformed EUI-48/64 value")

// validEUI reports whether s is exactly octets hyphen-separated 2-hex-digit
// groups (octets=6 for EUI-48, 8 for EUI-64).
func validEUI(s string, octets int) bool {
	parts := strings.Split(s, "-")
	if len(parts) != octets {
		return false
	}
	for _, p := range parts {
		if len(p) != 2 {
			return false
		}
		if _, err := strconv.ParseUint(p, 16, 8); err != nil {
			return false
		}
	}
	return true
}

// RTPMap models an a=rtpmap attribute (RFC 4566 §6):
// "<payload type> <encoding name>/<clock rate>[/<encoding parameters>]".
type RTPMap struct {
	PayloadType    uint8
	EncodingName   string
	ClockRate      uint32
	EncodingParams string // e.g. channel count for audio; empty if absent
}

// String renders the rtpmap value (without the leading "rtpmap:").
func (r RTPMap) String() string {
	s := strconv.Itoa(int(r.PayloadType)) + " " + r.EncodingName + "/" + strconv.FormatUint(uint64(r.ClockRate), 10)
	if r.EncodingParams != "" {
		s += "/" + r.EncodingParams
	}
	return s
}

// ParseRTPMap parses an rtpmap attribute value.
func ParseRTPMap(v string) (RTPMap, error) {
	pt, rest, ok := strings.Cut(strings.TrimSpace(v), " ")
	if !ok {
		return RTPMap{}, ErrBadAttribute
	}
	ptn, err := strconv.ParseUint(pt, 10, 7)
	if err != nil {
		return RTPMap{}, ErrBadAttribute
	}
	parts := strings.Split(strings.TrimSpace(rest), "/")
	if len(parts) < 2 {
		return RTPMap{}, ErrBadAttribute
	}
	rate, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return RTPMap{}, ErrBadAttribute
	}
	m := RTPMap{PayloadType: uint8(ptn), EncodingName: parts[0], ClockRate: uint32(rate)}
	if len(parts) >= 3 {
		m.EncodingParams = parts[2]
	}
	return m, nil
}

// PTP profile constant used by ST 2110-10 §8.2 examples.
const PTPProfileIEEE1588_2008 = "IEEE1588-2008"

// RefClock models an a=ts-refclk attribute (RFC 7273) as used by ST 2110-10
// §8.2. Exactly one of the PTP or LocalMAC forms is populated.
type RefClock struct {
	// PTP indicates the ts-refclk:ptp form.
	PTP bool
	// PTPProfile is the profile token, normally PTPProfileIEEE1588_2008.
	PTPProfile string
	// Traceable signals "ptp=<profile>:traceable" (RFC 7273 §4.7).
	Traceable bool
	// GMID is the grandmaster clockIdentity in EUI-64 form (hyphen-separated),
	// valid when PTP && !Traceable.
	GMID string
	// Domain is the PTP domain number, valid when PTP && !Traceable && !NoDomain.
	Domain int
	// DomainNmbr selects the RFC 7273 "domain-nmbr=<n>" rendering of the PTP
	// domain (used by ST 2110-10:2022). When false the bare ":<n>" form of
	// ST 2110-10:2017 is rendered. Parsing sets it from the form encountered.
	DomainNmbr bool
	// NoDomain indicates the PTP domain is omitted (RFC 7273 makes it optional).
	NoDomain bool

	// LocalMAC signals the "ts-refclk:localmac=<EUI-48>" form (ST 2110-10 §8.2).
	LocalMAC string
}

// String renders the ts-refclk value (without the leading "ts-refclk:").
func (r RefClock) String() string {
	if r.LocalMAC != "" {
		return "localmac=" + r.LocalMAC
	}
	profile := r.PTPProfile
	if profile == "" {
		profile = PTPProfileIEEE1588_2008
	}
	if r.Traceable {
		return "ptp=" + profile + ":traceable"
	}
	s := "ptp=" + profile + ":" + r.GMID
	if r.NoDomain {
		return s
	}
	if r.DomainNmbr {
		return s + ":domain-nmbr=" + strconv.Itoa(r.Domain)
	}
	return s + ":" + strconv.Itoa(r.Domain)
}

// ParseRefClock parses a ts-refclk attribute value.
func ParseRefClock(v string) (RefClock, error) {
	v = strings.TrimSpace(v)
	if mac, ok := strings.CutPrefix(v, "localmac="); ok {
		if !validEUI(mac, 6) { // EUI-48
			return RefClock{}, ErrBadEUI
		}
		return RefClock{LocalMAC: mac}, nil
	}
	body, ok := strings.CutPrefix(v, "ptp=")
	if !ok {
		return RefClock{}, ErrBadAttribute
	}
	// ptp = ptp-version ":" ptp-server, where ptp-server is "traceable" or an
	// EUI-64 clockIdentity optionally followed by ":" and a PTP domain.
	profile, rest, ok := strings.Cut(body, ":")
	if !ok {
		return RefClock{}, ErrBadAttribute
	}
	rc := RefClock{PTP: true, PTPProfile: profile}
	if rest == "traceable" {
		rc.Traceable = true
		return rc, nil
	}
	// The EUI-64 grandmaster id uses '-' separators (no ':'), so the first ':'
	// after it begins the optional domain.
	gmid, domainPart, hasDomain := strings.Cut(rest, ":")
	if !validEUI(gmid, 8) { // EUI-64
		return RefClock{}, ErrBadEUI
	}
	rc.GMID = gmid
	if !hasDomain {
		rc.NoDomain = true // RFC 7273: the PTP domain is optional
		return rc, nil
	}
	if n, ok := strings.CutPrefix(domainPart, "domain-nmbr="); ok {
		dom, err := strconv.Atoi(n)
		if err != nil {
			return RefClock{}, ErrBadAttribute
		}
		rc.Domain = dom
		rc.DomainNmbr = true
		return rc, nil
	}
	// Bare integer domain (ST 2110-10:2017 form).
	dom, err := strconv.Atoi(domainPart)
	if err != nil {
		return RefClock{}, ErrBadAttribute
	}
	rc.Domain = dom
	return rc, nil
}

// MediaClock models an a=mediaclk attribute (RFC 7273 §5) as used by ST 2110-10
// §8.3: "mediaclk:direct=<offset>" or "mediaclk:sender".
type MediaClock struct {
	// Direct selects the direct-reference form; Offset is its value (zero in
	// ST 2110-10 §7.3).
	Direct bool
	Offset uint64
	// Sender selects the asynchronous "mediaclk:sender" form.
	Sender bool
	// ID is the optional "id=<media-clktag>" master-clock identifier prefix
	// (RFC 7273 §5.3), e.g. "src:word1" or "stream7". Empty when absent.
	ID string
	// RateNum/RateDen carry the optional "rate=<num>/<den>" modifier of the
	// direct form (RFC 7273 §5.2). RateNum == 0 means the modifier is absent.
	RateNum int64
	RateDen int64
}

// String renders the mediaclk value (without the leading "mediaclk:").
func (m MediaClock) String() string {
	var prefix string
	if m.ID != "" {
		prefix = "id=" + m.ID + " "
	}
	if m.Sender {
		return prefix + "sender"
	}
	s := prefix + "direct=" + strconv.FormatUint(m.Offset, 10)
	if m.RateNum != 0 {
		s += " rate=" + strconv.FormatInt(m.RateNum, 10) + "/" + strconv.FormatInt(m.RateDen, 10)
	}
	return s
}

// ParseMediaClock parses a mediaclk attribute value, including the optional
// "id=<tag>" master-clock prefix (RFC 7273 §5.3) and the "rate=<num>/<den>"
// modifier of the direct form (§5.2).
func ParseMediaClock(v string) (MediaClock, error) {
	v = strings.TrimSpace(v)
	var mc MediaClock
	if after, ok := strings.CutPrefix(v, "id="); ok {
		id, rest, ok := strings.Cut(after, " ")
		if !ok {
			return MediaClock{}, ErrBadAttribute
		}
		mc.ID = id
		v = strings.TrimSpace(rest)
	}
	if v == "sender" {
		mc.Sender = true
		return mc, nil
	}
	fields := strings.Fields(v)
	if len(fields) == 0 {
		return MediaClock{}, ErrBadAttribute
	}
	off, ok := strings.CutPrefix(fields[0], "direct=")
	if !ok {
		return MediaClock{}, ErrBadAttribute
	}
	n, err := strconv.ParseUint(off, 10, 64)
	if err != nil {
		return MediaClock{}, ErrBadAttribute
	}
	mc.Direct = true
	mc.Offset = n
	for _, f := range fields[1:] {
		rate, ok := strings.CutPrefix(f, "rate=")
		if !ok {
			return MediaClock{}, ErrBadAttribute
		}
		numStr, denStr, ok := strings.Cut(rate, "/")
		if !ok {
			return MediaClock{}, ErrBadAttribute
		}
		num, err1 := strconv.ParseInt(numStr, 10, 64)
		den, err2 := strconv.ParseInt(denStr, 10, 64)
		if err1 != nil || err2 != nil || num <= 0 || den <= 0 {
			return MediaClock{}, ErrBadAttribute
		}
		mc.RateNum, mc.RateDen = num, den
	}
	return mc, nil
}

// SourceFilter models an a=source-filter attribute (RFC 4570):
// "<mode> <nettype> <addrtype> <dest-address> <src-list>".
type SourceFilter struct {
	Mode        string // "incl" or "excl"
	NetType     string // "IN"
	AddrType    string // "IP4"/"IP6"
	DestAddress string
	Sources     []string
}

// String renders the source-filter value (without "source-filter:"). RFC 4570
// allows a leading space after the colon, matching the ST 2110-10 examples.
func (s SourceFilter) String() string {
	parts := []string{s.Mode, s.NetType, s.AddrType, s.DestAddress}
	parts = append(parts, s.Sources...)
	return " " + strings.Join(parts, " ")
}

// ParseSourceFilter parses a source-filter attribute value.
func ParseSourceFilter(v string) (SourceFilter, error) {
	f := strings.Fields(v)
	if len(f) < 5 {
		return SourceFilter{}, ErrBadAttribute
	}
	return SourceFilter{
		Mode: f[0], NetType: f[1], AddrType: f[2], DestAddress: f[3], Sources: f[4:],
	}, nil
}

// Group models an a=group attribute (RFC 5888) such as the DUP grouping of
// RFC 7104 used by ST 2110-10 §8.5: "DUP primary secondary".
type Group struct {
	Semantics string
	MIDs      []string
}

// String renders the group value (without "group:").
func (g Group) String() string {
	return strings.Join(append([]string{g.Semantics}, g.MIDs...), " ")
}

// ParseGroup parses a group attribute value.
func ParseGroup(v string) (Group, error) {
	f := strings.Fields(v)
	if len(f) < 1 {
		return Group{}, ErrBadAttribute
	}
	return Group{Semantics: f[0], MIDs: f[1:]}, nil
}

// --- Convenience accessors on the SDP model ---

// RTPMap returns the parsed rtpmap on the media, if present.
func (m *MediaDescription) RTPMap() (RTPMap, bool) {
	if v, ok := m.GetAttribute("rtpmap"); ok {
		if r, err := ParseRTPMap(v); err == nil {
			return r, true
		}
	}
	return RTPMap{}, false
}

// RefClock returns the parsed ts-refclk on the media, if present.
func (m *MediaDescription) RefClock() (RefClock, bool) {
	if v, ok := m.GetAttribute("ts-refclk"); ok {
		if r, err := ParseRefClock(v); err == nil {
			return r, true
		}
	}
	return RefClock{}, false
}

// MediaClock returns the parsed mediaclk on the media, if present.
func (m *MediaDescription) MediaClock() (MediaClock, bool) {
	if v, ok := m.GetAttribute("mediaclk"); ok {
		if c, err := ParseMediaClock(v); err == nil {
			return c, true
		}
	}
	return MediaClock{}, false
}

// MID returns the a=mid value on the media, if present.
func (m *MediaDescription) MID() (string, bool) {
	return m.GetAttribute("mid")
}

// SetAttribute appends a value-bearing a= attribute to the media.
func (m *MediaDescription) SetAttribute(key, value string) {
	m.Attributes = append(m.Attributes, Attribute{Key: key, Value: value, HasValue: true})
}

// SetFlag appends a value-less a= attribute to the media.
func (m *MediaDescription) SetFlag(key string) {
	m.Attributes = append(m.Attributes, Attribute{Key: key})
}
