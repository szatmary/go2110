package timing21

import (
	"math"
	"strconv"

	"github.com/szatmary/go2110/sdp"
)

// SetTP sets the ST 2110-21 TP media-type parameter (§7.1) for the sender type
// on the given fmtp parameters.
func SetTP(fp *sdp.FormatParameters, st SenderType) {
	fp.Set("TP", st.TP())
}

// SetTROFF sets the TROFF media-type parameter (ST 2110-21 §8.2), signalling the
// prevailing TROFFSET. Per §8.2 the value is "a positive integer number of
// microseconds"; troffSeconds (TROFFSET in seconds) is rounded to the nearest
// microsecond. It is NOT expressed in RTP-clock ticks.
func SetTROFF(fp *sdp.FormatParameters, troffSeconds float64) {
	us := int64(math.Round(troffSeconds * 1e6))
	fp.Set("TROFF", strconv.FormatInt(us, 10))
}

// ParseTROFF reads the TROFF media-type parameter (microseconds, §8.2) and
// returns the TROFFSET in seconds. The second result is false when TROFF is
// absent or malformed.
func ParseTROFF(fp sdp.FormatParameters) (float64, bool) {
	v, ok := fp.Get("TROFF")
	if !ok {
		return 0, false
	}
	us, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, false
	}
	return float64(us) / 1e6, true
}

// ParseSenderType reads the TP media-type parameter and returns the sender type.
func ParseSenderType(fp sdp.FormatParameters) (SenderType, bool) {
	v, ok := fp.Get("TP")
	if !ok {
		return 0, false
	}
	switch v {
	case "2110TPN":
		return TypeN, true
	case "2110TPNL":
		return TypeNL, true
	case "2110TPW":
		return TypeW, true
	}
	return 0, false
}
