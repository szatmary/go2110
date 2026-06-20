package video

import (
	"strconv"

	"github.com/szatmary/go2110/sdp"
	"github.com/szatmary/go2110/st2110/media"
)

// EncodingName is the rtpmap encoding name for ST 2110-20 video (§7.1: media
// subtype "raw").
const EncodingName = "raw"

// VideoClockRate is the RTP clock rate for ST 2110-20 (§6.1.3): 90 kHz.
const VideoClockRate = media.VideoClockRate

// RTPMap returns the a=rtpmap descriptor for the given dynamic payload type
// (ST 2110-20 §7.1: "raw/90000").
func RTPMap(payloadType uint8) sdp.RTPMap {
	return sdp.RTPMap{PayloadType: payloadType, EncodingName: EncodingName, ClockRate: VideoClockRate}
}

// FMTP builds the a=fmtp media-type parameters for the format (ST 2110-20 §7).
// Required parameters (sampling, width, height, exactframerate, depth,
// colorimetry, PM, SSN) are always emitted; TCS and the optional parameters are
// emitted when set. The parameter order matches the §7.7 informative example.
func (f Format) FMTP(payloadType uint8) sdp.FormatParameters {
	fp := sdp.FormatParameters{Format: strconv.Itoa(int(payloadType))}
	fp.Set("sampling", string(f.Sampling))
	fp.Set("width", strconv.Itoa(f.Width))
	fp.Set("height", strconv.Itoa(f.Height))
	fp.Set("exactframerate", f.ExactFrameRate.String())
	if f.Interlaced {
		fp.SetFlag("interlace")
	}
	if f.Segmented {
		fp.SetFlag("segmented")
	}
	fp.Set("depth", string(f.Depth))
	if f.TCS != "" {
		fp.Set("TCS", f.TCS)
	}
	// colorimetry is a Required media-type parameter (ST 2110-20 §7.2); it must
	// always appear. When unset, default to BT709 (the common HD colorimetry)
	// rather than emitting a non-conformant colorimetry-less fmtp.
	colorimetry := f.Colorimetry
	if colorimetry == "" {
		colorimetry = "BT709"
	}
	fp.Set("colorimetry", colorimetry)
	if f.Range != "" {
		fp.Set("RANGE", f.Range)
	}
	pm := f.PackingMode
	if pm == "" {
		pm = PackingGPM
	}
	fp.Set("PM", string(pm))
	ssn := f.SSN
	if ssn == "" {
		ssn = "ST2110-20:2017"
	}
	fp.Set("SSN", ssn)
	if f.MaxUDP > 0 {
		fp.Set("MAXUDP", strconv.Itoa(f.MaxUDP))
	}
	if f.PAR != "" {
		fp.Set("PAR", f.PAR)
	}
	return fp
}

// ParseFMTP parses ST 2110-20 video media-type parameters into a Format.
func ParseFMTP(fp sdp.FormatParameters) (Format, error) {
	var f Format
	if v, ok := fp.Get("sampling"); ok {
		f.Sampling = Sampling(v)
	}
	if v, ok := fp.Get("depth"); ok {
		f.Depth = Depth(v)
	}
	if v, ok := fp.Get("width"); ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Format{}, err
		}
		f.Width = n
	}
	if v, ok := fp.Get("height"); ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Format{}, err
		}
		f.Height = n
	}
	if v, ok := fp.Get("exactframerate"); ok {
		r, err := media.ParseExactFrameRate(v)
		if err != nil {
			return Format{}, err
		}
		f.ExactFrameRate = r
	}
	f.Interlaced = fp.Has("interlace")
	f.Segmented = fp.Has("segmented")
	if v, ok := fp.Get("TCS"); ok {
		f.TCS = v
	}
	if v, ok := fp.Get("colorimetry"); ok {
		f.Colorimetry = v
	}
	if v, ok := fp.Get("RANGE"); ok {
		f.Range = v
	}
	if v, ok := fp.Get("PM"); ok {
		f.PackingMode = PackingMode(v)
	}
	if v, ok := fp.Get("SSN"); ok {
		f.SSN = v
	}
	if v, ok := fp.Get("MAXUDP"); ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Format{}, err
		}
		f.MaxUDP = n
	}
	if v, ok := fp.Get("PAR"); ok {
		f.PAR = v
	}
	// Validate that the sampling/depth combination is known.
	if _, err := f.Pgroup(); err != nil {
		return Format{}, err
	}
	return f, nil
}
