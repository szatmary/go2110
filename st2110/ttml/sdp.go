package ttml

import (
	"strconv"

	"github.com/szatmary/go2110/sdp"
)

// EncodingName is the rtpmap encoding name for TTML over RTP (RFC 8759 §11:
// media type "application/ttml+xml").
const EncodingName = "ttml+xml"

// RTPMap returns the a=rtpmap descriptor: "ttml+xml/90000" (ST 2110-43 §4.2).
func RTPMap(payloadType uint8) sdp.RTPMap {
	return sdp.RTPMap{PayloadType: payloadType, EncodingName: EncodingName, ClockRate: RTPClockRate}
}

// DefaultProfile is the default TTML2 processor profile short code emitted in the
// "codecs" fmtp parameter when none is supplied: "im2t" (IMSC 1.1 Text), as used
// in the RFC 8759 §11.2.1 example SDP.
const DefaultProfile = "im2t"

// FMTP returns the mandatory a=fmtp value for the TTML stream, matching the
// RFC 8759 §11.2.1 example exactly: "<pt> charset=utf-8;codecs=<profile>".
// RFC 8759 §6.1.3 / §11.2 make the "codecs" parameter a MUST. An empty profile
// defaults to DefaultProfile. The parameters are joined with a bare ";" (no
// surrounding space) so the codecs token parses cleanly per the media-type
// registration.
func FMTP(payloadType uint8, profile string) string {
	if profile == "" {
		profile = DefaultProfile
	}
	return strconv.Itoa(int(payloadType)) + " charset=utf-8;codecs=" + profile
}

// MediaDescription builds the SDP media section for the TTML stream (media type
// "application"), per RFC 8759 §11.2, including the mandatory a=fmtp line with
// charset and codecs (processor profile). An empty profile defaults to
// DefaultProfile ("im2t").
func MediaDescription(port int, payloadType uint8, profile string) *sdp.MediaDescription {
	m := &sdp.MediaDescription{
		Media:    "application",
		Port:     port,
		Protocol: "RTP/AVP",
		Formats:  []string{strconv.Itoa(int(payloadType))},
	}
	m.SetAttribute("rtpmap", RTPMap(payloadType).String())
	m.SetAttribute("fmtp", FMTP(payloadType, profile))
	return m
}
