package aes3

import (
	"math"
	"strconv"
	"time"

	"github.com/szatmary/go2110/sdp"
)

// EncodingName is the rtpmap encoding name for ST 2110-31 (§6.1: media subtype
// "AM824").
const EncodingName = "AM824"

// RTPMap returns the a=rtpmap descriptor: "AM824/<clock-rate>/<nchan>"
// (ST 2110-31 §6.1).
func (f Format) RTPMap(payloadType uint8) sdp.RTPMap {
	return sdp.RTPMap{
		PayloadType:    payloadType,
		EncodingName:   EncodingName,
		ClockRate:      uint32(f.SampleRate),
		EncodingParams: strconv.Itoa(f.Channels),
	}
}

// PTime returns the a=ptime value in milliseconds, matching the permitted values
// of ST 2110-31 Table 1. Per the Table 3 Note the signalled packet time is
// rounded to two decimal places with midway values rounded down, so a 125 µs
// packet time is signalled as "0.12" (not "0.125"), a 44.1 kHz 6-period packet as
// "0.14", and so on. Trailing zeros are trimmed (1 ms → "1").
func (f Format) PTime() string {
	ms := float64(f.PacketTime) / float64(time.Millisecond)
	// Round half down to two decimal places (ceil(x-0.5) breaks ties downward).
	rounded := math.Ceil(ms*100-0.5) / 100
	return strconv.FormatFloat(rounded, 'g', -1, 64)
}

// MediaDescription builds an SDP media section for the AES3 stream.
func (f Format) MediaDescription(port int, payloadType uint8) *sdp.MediaDescription {
	m := &sdp.MediaDescription{
		Media:    "audio",
		Port:     port,
		Protocol: "RTP/AVP",
		Formats:  []string{strconv.Itoa(int(payloadType))},
	}
	m.SetAttribute("rtpmap", f.RTPMap(payloadType).String())
	m.SetAttribute("ptime", f.PTime())
	return m
}

// ParseMediaDescription extracts an AES3 Format from an SDP media section.
func ParseMediaDescription(m *sdp.MediaDescription) (Format, error) {
	var f Format
	rm, ok := m.RTPMap()
	if !ok || rm.EncodingName != EncodingName {
		return Format{}, ErrBadRate
	}
	f.SampleRate = int(rm.ClockRate)
	if rm.EncodingParams != "" {
		n, err := strconv.Atoi(rm.EncodingParams)
		if err != nil {
			return Format{}, ErrBadChannels
		}
		f.Channels = n
	}
	if v, ok := m.GetAttribute("ptime"); ok {
		ms, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return Format{}, err
		}
		f.PacketTime = time.Duration(ms * float64(time.Millisecond))
	}
	return f, nil
}
