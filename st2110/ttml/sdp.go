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

// MediaDescription builds the SDP media section for the TTML stream (media type
// "application"), per RFC 8759 §11.2.
func MediaDescription(port int, payloadType uint8) *sdp.MediaDescription {
	m := &sdp.MediaDescription{
		Media:    "application",
		Port:     port,
		Protocol: "RTP/AVP",
		Formats:  []string{strconv.Itoa(int(payloadType))},
	}
	m.SetAttribute("rtpmap", RTPMap(payloadType).String())
	return m
}
