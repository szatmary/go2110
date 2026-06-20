// Package cv22 implements SMPTE ST 2110-22 "Constant Bit-Rate Compressed Video".
//
// ST 2110-22 is a transport and signalling framework: it does not define a
// payload header of its own but carries a constant-bit-rate compressed video
// stream (e.g. JPEG XS "jxsv", VC-2 "vc2") whose own RTP payload format is
// defined elsewhere. The standard requires a constant number of bytes and RTP
// packets per frame (§4), a 90 kHz RTP clock (§5.2), ST 2110-21 traffic shaping
// signalled with TP (§5.3), and an SDP object carrying the compressed subtype,
// width/height, TP, bit rate (b=AS) and frame rate (§7).
//
// This package builds and parses that SDP and provides the constant-frame /
// bit-rate helpers; the inner compressed-payload codec is out of scope.
package cv22

import (
	"errors"
	"strconv"

	"github.com/szatmary/go2110/sdp"
	"github.com/szatmary/go2110/st2110/media"
	"github.com/szatmary/go2110/st2110/timing21"
)

// RTPClockRate is the ST 2110-22 RTP timestamp clock rate (§5.2): 90 kHz.
const RTPClockRate = 90000

// ErrMissingParam is returned when a required SDP parameter is absent.
var ErrMissingParam = errors.New("st2110/cv22: missing required media-type parameter")

// Format describes an ST 2110-22 compressed video stream's SDP signalling.
type Format struct {
	// Subtype is the rtpmap encoding name of the compressed format, e.g. "jxsv"
	// (JPEG XS) or "vc2" (VC-2).
	Subtype string
	// Width and Height are the image dimensions (Table 1, mandatory).
	Width  int
	Height int
	// FrameRate is emitted as the a=framerate attribute and/or exactframerate.
	FrameRate media.Rational
	// SenderType is the ST 2110-21 sender type signalled with TP (Table 1).
	SenderType timing21.SenderType
	// CMax optionally signals the ST 2110-21 CMAX value (Table 2; 0 ⇒ omit).
	CMax int
	// SSN optionally signals the SMPTE Standard Number (Table 2).
	SSN string
	// BitrateKbps is the average bit rate for b=AS in kilobits/second (§7.3).
	BitrateKbps int
	// Extra holds additional codec-specific fmtp parameters (e.g. profile, level)
	// emitted before the standard ST 2110-22 parameters, matching the §7.5
	// example ordering.
	Extra []sdp.Param
}

// BitrateKbpsFor computes the b=AS value (kilobits/second, rounded up) from the
// constant coded frame size in octets and the frame rate, per ST 2110-22 §7.3.
// The size should include the whole of each IP packet (IP + UDP + RTP + payload).
func BitrateKbpsFor(bytesPerFrame int, frameRate media.Rational) int {
	// bits/sec = bytesPerFrame*8 * (Num/Den); kbps = ceil(bits/sec / 1000).
	bits := int64(bytesPerFrame) * 8 * frameRate.Num
	den := int64(1000) * frameRate.Den
	return int((bits + den - 1) / den)
}

// frameRateAttr renders the a=framerate value: an integer when Den==1, otherwise
// a decimal approximation (RFC 4566 permits <integer>.<fraction>).
func (f Format) frameRateAttr() string {
	if f.FrameRate.Den == 1 {
		return strconv.FormatInt(f.FrameRate.Num, 10)
	}
	return strconv.FormatFloat(f.FrameRate.Float64(), 'g', -1, 64)
}

// RTPMap returns the a=rtpmap descriptor: "<subtype>/90000".
func (f Format) RTPMap(payloadType uint8) sdp.RTPMap {
	return sdp.RTPMap{PayloadType: payloadType, EncodingName: f.Subtype, ClockRate: RTPClockRate}
}

// FMTP builds the a=fmtp parameters: any Extra codec parameters first, then
// width, height, TP, and optionally CMAX and SSN (matching the §7.5 example).
func (f Format) FMTP(payloadType uint8) sdp.FormatParameters {
	fp := sdp.FormatParameters{Format: strconv.Itoa(int(payloadType))}
	fp.Params = append(fp.Params, f.Extra...)
	fp.Set("width", strconv.Itoa(f.Width))
	fp.Set("height", strconv.Itoa(f.Height))
	fp.Set("TP", f.SenderType.TP())
	if f.CMax > 0 {
		fp.Set("CMAX", strconv.Itoa(f.CMax))
	}
	if f.SSN != "" {
		fp.Set("SSN", f.SSN)
	}
	return fp
}

// MediaDescription builds the SDP media section for the compressed stream,
// including b=AS, rtpmap, a=framerate and fmtp (§7.5). Session-level and
// ST 2110-10 attributes (c=, ts-refclk, mediaclk) are the caller's
// responsibility.
func (f Format) MediaDescription(port int, payloadType uint8) *sdp.MediaDescription {
	m := &sdp.MediaDescription{
		Media:    "video",
		Port:     port,
		Protocol: "RTP/AVP",
		Formats:  []string{strconv.Itoa(int(payloadType))},
	}
	if f.BitrateKbps > 0 {
		m.Bandwidth = append(m.Bandwidth, sdp.Bandwidth{Type: "AS", Value: uint64(f.BitrateKbps)})
	}
	m.SetAttribute("rtpmap", f.RTPMap(payloadType).String())
	if f.FrameRate.Num != 0 {
		m.SetAttribute("framerate", f.frameRateAttr())
	}
	m.SetAttribute("fmtp", f.FMTP(payloadType).String())
	return m
}

// ParseMediaDescription extracts an ST 2110-22 Format from an SDP media section.
func ParseMediaDescription(m *sdp.MediaDescription) (Format, error) {
	var f Format
	rm, ok := m.RTPMap()
	if !ok {
		return Format{}, ErrMissingParam
	}
	f.Subtype = rm.EncodingName
	for _, b := range m.Bandwidth {
		if b.Type == "AS" {
			f.BitrateKbps = int(b.Value)
		}
	}
	if v, ok := m.GetAttribute("framerate"); ok {
		if r, err := media.ParseExactFrameRate(v); err == nil {
			f.FrameRate = r
		}
	}
	fp, ok := m.FormatParameters()
	if !ok {
		return Format{}, ErrMissingParam
	}
	if v, ok := fp.Get("width"); ok {
		f.Width, _ = strconv.Atoi(v)
	}
	if v, ok := fp.Get("height"); ok {
		f.Height, _ = strconv.Atoi(v)
	}
	if v, ok := fp.Get("TP"); ok {
		switch v {
		case "2110TPN":
			f.SenderType = timing21.TypeN
		case "2110TPNL":
			f.SenderType = timing21.TypeNL
		case "2110TPW":
			f.SenderType = timing21.TypeW
		}
	}
	if v, ok := fp.Get("CMAX"); ok {
		f.CMax, _ = strconv.Atoi(v)
	}
	if v, ok := fp.Get("SSN"); ok {
		f.SSN = v
	}
	// Preserve codec-specific parameters (anything that isn't an ST 2110-22 one).
	known := map[string]bool{"width": true, "height": true, "TP": true, "CMAX": true, "SSN": true}
	for _, p := range fp.Params {
		if !known[p.Name] {
			f.Extra = append(f.Extra, p)
		}
	}
	return f, nil
}
