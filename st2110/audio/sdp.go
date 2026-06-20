package audio

import (
	"strconv"
	"time"

	"github.com/szatmary/go2110/sdp"
)

// RTPMap returns the a=rtpmap descriptor for the audio format (AES67 / RFC 3190):
// "<pt> <encoding>/<sample-rate>/<channels>", e.g. "101 L24/48000/2".
func (f Format) RTPMap(payloadType uint8) sdp.RTPMap {
	return sdp.RTPMap{
		PayloadType:    payloadType,
		EncodingName:   string(f.Encoding),
		ClockRate:      uint32(f.SampleRate),
		EncodingParams: strconv.Itoa(f.Channels),
	}
}

// PTime returns the a=ptime attribute value in milliseconds (RFC 4566), e.g.
// "1" for 1 ms or "0.125" for 125 µs.
func (f Format) PTime() string {
	ms := float64(f.PacketTime) / float64(time.Millisecond)
	return strconv.FormatFloat(ms, 'g', -1, 64)
}

// FMTP returns the a=fmtp parameters for the audio format. Only the channel-order
// parameter is defined by ST 2110-30 (§6.2.2); it is emitted when set.
func (f Format) FMTP(payloadType uint8) sdp.FormatParameters {
	fp := sdp.FormatParameters{Format: strconv.Itoa(int(payloadType))}
	if f.ChannelOrder != "" {
		fp.Set("channel-order", f.ChannelOrder)
	}
	return fp
}

// MediaDescription builds a complete SDP media section for the audio stream on
// the given UDP port and dynamic payload type, including rtpmap, ptime, and (if
// a channel order is set) fmtp.
func (f Format) MediaDescription(port int, payloadType uint8) *sdp.MediaDescription {
	m := &sdp.MediaDescription{
		Media:    "audio",
		Port:     port,
		Protocol: "RTP/AVP",
		Formats:  []string{strconv.Itoa(int(payloadType))},
	}
	m.SetAttribute("rtpmap", f.RTPMap(payloadType).String())
	if f.ChannelOrder != "" {
		m.SetAttribute("fmtp", f.FMTP(payloadType).String())
	}
	m.SetAttribute("ptime", f.PTime())
	return m
}

// ParseMediaDescription extracts an audio Format from an SDP media section,
// reading the rtpmap (encoding/rate/channels), ptime, and channel-order.
func ParseMediaDescription(m *sdp.MediaDescription) (Format, error) {
	var f Format
	rm, ok := m.RTPMap()
	if !ok {
		return Format{}, ErrUnsupportedEncoding
	}
	f.Encoding = Encoding(rm.EncodingName)
	f.SampleRate = int(rm.ClockRate)
	if rm.EncodingParams != "" {
		n, err := strconv.Atoi(rm.EncodingParams)
		if err != nil {
			return Format{}, ErrBadChannelCount
		}
		f.Channels = n
	} else {
		f.Channels = 1 // RFC 3190: absent channel count defaults to 1
	}
	if v, ok := m.GetAttribute("ptime"); ok {
		ms, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return Format{}, ErrBadPacketTime
		}
		f.PacketTime = time.Duration(ms * float64(time.Millisecond))
	}
	if fp, ok := m.FormatParameters(); ok {
		if v, ok := fp.Get("channel-order"); ok {
			f.ChannelOrder = v
		}
	}
	return f, nil
}
