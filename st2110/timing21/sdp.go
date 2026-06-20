package timing21

import (
	"strconv"

	"github.com/szatmary/go2110/sdp"
)

// SetTP sets the ST 2110-21 TP media-type parameter (§7.1) for the sender type
// on the given fmtp parameters.
func SetTP(fp *sdp.FormatParameters, st SenderType) {
	fp.Set("TP", st.TP())
}

// SetTROFF sets the TROFF media-type parameter (ST 2110-21 §6.2), signalling the
// prevailing TROFFSET. The value is the TROFFSET expressed in units of the
// stream's RTP clock, as an integer.
func SetTROFF(fp *sdp.FormatParameters, troff int) {
	fp.Set("TROFF", strconv.Itoa(troff))
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
