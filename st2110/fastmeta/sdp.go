package fastmeta

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/szatmary/go2110/sdp"
)

// EncodingName is the rtpmap encoding name for ST 2110-41 (§9.2.1: media type
// "application/ST2110-41").
const EncodingName = "ST2110-41"

// Format describes the SDP signalling of an ST 2110-41 fast metadata stream.
type Format struct {
	// ClockRate is the RTP timestamp clock rate (§5.3, signalled in rtpmap).
	ClockRate uint32
	// SSN is the SMPTE Standard Number (§6: "ST2110-41:2024").
	SSN string
	// DIT is the optional list of Data Item Types that may appear (§6).
	DIT []uint32
}

// RTPMap returns the a=rtpmap descriptor: "ST2110-41/<rate>".
func (f Format) RTPMap(payloadType uint8) sdp.RTPMap {
	return sdp.RTPMap{PayloadType: payloadType, EncodingName: EncodingName, ClockRate: f.ClockRate}
}

// ditString renders the DIT value: uppercase hexadecimal, no "0x" prefix,
// comma-separated, no whitespace (§6).
func ditString(dit []uint32) string {
	parts := make([]string, len(dit))
	for i, d := range dit {
		parts[i] = fmt.Sprintf("%X", d)
	}
	return strings.Join(parts, ",")
}

// FMTP builds the a=fmtp parameters (§6): SSN (required) and DIT (optional).
func (f Format) FMTP(payloadType uint8) sdp.FormatParameters {
	fp := sdp.FormatParameters{Format: strconv.Itoa(int(payloadType))}
	if f.SSN != "" {
		fp.Set("SSN", f.SSN)
	}
	if len(f.DIT) > 0 {
		fp.Set("DIT", ditString(f.DIT))
	}
	return fp
}

// MediaDescription builds the SDP media section (media type "application").
func (f Format) MediaDescription(port int, payloadType uint8) *sdp.MediaDescription {
	m := &sdp.MediaDescription{
		Media:    "application",
		Port:     port,
		Protocol: "RTP/AVP",
		Formats:  []string{strconv.Itoa(int(payloadType))},
	}
	m.SetAttribute("rtpmap", f.RTPMap(payloadType).String())
	m.SetAttribute("fmtp", f.FMTP(payloadType).String())
	return m
}

// ParseMediaDescription extracts an ST 2110-41 Format from an SDP media section.
func ParseMediaDescription(m *sdp.MediaDescription) (Format, error) {
	var f Format
	rm, ok := m.RTPMap()
	if !ok {
		return Format{}, ErrShortPayload
	}
	f.ClockRate = rm.ClockRate
	if fp, ok := m.FormatParameters(); ok {
		if v, ok := fp.Get("SSN"); ok {
			f.SSN = v
		}
		if v, ok := fp.Get("DIT"); ok {
			for _, h := range strings.Split(v, ",") {
				if h == "" {
					continue
				}
				n, err := strconv.ParseUint(h, 16, 32)
				if err != nil {
					return Format{}, err
				}
				f.DIT = append(f.DIT, uint32(n))
			}
		}
	}
	return f, nil
}
