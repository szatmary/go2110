package anc

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/szatmary/go2110/sdp"
	"github.com/szatmary/go2110/st2110/media"
)

// EncodingName is the rtpmap encoding name for ST 2110-40 ANC data (RFC 8331 §5:
// media type "video/smpte291").
const EncodingName = "smpte291"

// DIDSDID is a DID/SDID pair signalled with the RFC 8331 DID_SDID media-type
// parameter, advertising which ANC data types a stream may carry.
type DIDSDID struct {
	DID  uint8
	SDID uint8
}

// String renders a DID_SDID value body, e.g. "{0x61,0x02}".
func (d DIDSDID) String() string {
	return fmt.Sprintf("{0x%02x,0x%02x}", d.DID, d.SDID)
}

// ParseDIDSDID parses a DID_SDID value body such as "{0x61,0x02}".
func ParseDIDSDID(v string) (DIDSDID, error) {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "{") || !strings.HasSuffix(v, "}") {
		return DIDSDID{}, ErrFieldRange
	}
	did, sdid, ok := strings.Cut(v[1:len(v)-1], ",")
	if !ok {
		return DIDSDID{}, ErrFieldRange
	}
	d, err1 := strconv.ParseUint(strings.TrimPrefix(strings.TrimSpace(did), "0x"), 16, 8)
	s, err2 := strconv.ParseUint(strings.TrimPrefix(strings.TrimSpace(sdid), "0x"), 16, 8)
	if err1 != nil || err2 != nil {
		return DIDSDID{}, ErrFieldRange
	}
	return DIDSDID{DID: uint8(d), SDID: uint8(s)}, nil
}

// Format describes the SDP signalling of an ST 2110-40 ANC stream (§7).
type Format struct {
	// ExactFrameRate is the §7 exactframerate (required) of the related video.
	ExactFrameRate media.Rational
	// DIDSDIDs optionally restricts the ANC data types carried (RFC 8331).
	DIDSDIDs []DIDSDID
	// VPIDCode is the optional VPID_Code (0 ⇒ absent).
	VPIDCode int
	// TM is the optional transmission model ("CTM" or "LLTM").
	TM string
	// SSN is the SMPTE Standard Number (e.g. "ST2110-40:2018").
	SSN string
}

// RTPMap returns the a=rtpmap descriptor for the ANC stream: "smpte291/90000".
func RTPMap(payloadType uint8) sdp.RTPMap {
	return sdp.RTPMap{PayloadType: payloadType, EncodingName: EncodingName, ClockRate: RTPClockRate}
}

// FMTP builds the a=fmtp media-type parameters for the ANC stream (§7, RFC 8331).
func (f Format) FMTP(payloadType uint8) sdp.FormatParameters {
	fp := sdp.FormatParameters{Format: strconv.Itoa(int(payloadType))}
	if f.ExactFrameRate.Num != 0 {
		fp.Set("exactframerate", f.ExactFrameRate.String())
	}
	for _, d := range f.DIDSDIDs {
		fp.Params = append(fp.Params, sdp.Param{Name: "DID_SDID", Value: d.String(), HasValue: true})
	}
	if f.VPIDCode != 0 {
		fp.Set("VPID_Code", strconv.Itoa(f.VPIDCode))
	}
	if f.TM != "" {
		fp.Set("TM", f.TM)
	}
	if f.SSN != "" {
		fp.Set("SSN", f.SSN)
	}
	return fp
}

// MediaDescription builds a complete SDP media section for the ANC stream. Per
// RFC 8331 the media type is "video".
func (f Format) MediaDescription(port int, payloadType uint8) *sdp.MediaDescription {
	m := &sdp.MediaDescription{
		Media:    "video",
		Port:     port,
		Protocol: "RTP/AVP",
		Formats:  []string{strconv.Itoa(int(payloadType))},
	}
	m.SetAttribute("rtpmap", RTPMap(payloadType).String())
	m.SetAttribute("fmtp", f.FMTP(payloadType).String())
	return m
}

// ParseFMTP extracts an ANC Format from fmtp media-type parameters.
func ParseFMTP(fp sdp.FormatParameters) (Format, error) {
	var f Format
	if v, ok := fp.Get("exactframerate"); ok {
		r, err := media.ParseExactFrameRate(v)
		if err != nil {
			return Format{}, err
		}
		f.ExactFrameRate = r
	}
	for _, p := range fp.Params {
		if p.Name == "DID_SDID" && p.HasValue {
			d, err := ParseDIDSDID(p.Value)
			if err != nil {
				return Format{}, err
			}
			f.DIDSDIDs = append(f.DIDSDIDs, d)
		}
	}
	if v, ok := fp.Get("VPID_Code"); ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Format{}, err
		}
		f.VPIDCode = n
	}
	if v, ok := fp.Get("TM"); ok {
		f.TM = v
	}
	if v, ok := fp.Get("SSN"); ok {
		f.SSN = v
	}
	return f, nil
}
