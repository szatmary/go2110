// Package timing21 implements the SMPTE ST 2110-21 sender traffic-shaping models:
// the Packet Read Schedules (gapped and linear, §6.3/§6.4), the Network
// Compatibility leaky-bucket model (§6.6.1) and the Virtual Receiver Buffer
// model (§6.6.2), and the Narrow (N), Narrow Linear (NL), and Wide (W) sender
// type definitions of §7.1 with their VRXFULL and CMAX parameters and TP SDP
// signalling.
//
// The VRXFULL and CMAX expressions are evaluated with exact integer arithmetic
// (the standard's INT(...) is floor) so the results match the standard at the
// floor boundaries without floating-point error.
package timing21

import "github.com/szatmary/go2110/st2110/media"

// Beta is the scaling factor β applied to RNOMINAL in the Network Compatibility
// Model for all sender types (ST 2110-21 §7.1): 1.10.
const Beta = 1.10

// SenderType identifies an ST 2110-21 §7.1 sender type.
type SenderType int

// Sender types defined in ST 2110-21 §7.1.
const (
	TypeN  SenderType = iota // Narrow (gapped PRS), §7.1.2
	TypeNL                   // Narrow Linear (linear PRS), §7.1.3
	TypeW                    // Wide (linear PRS), §7.1.4
)

// PRSModel identifies a Packet Read Schedule model.
type PRSModel int

// Packet Read Schedule models (ST 2110-21 §6.3/§6.4).
const (
	Gapped PRSModel = iota
	Linear
)

// PRS returns the Packet Read Schedule used by the sender type (§7.1).
func (st SenderType) PRS() PRSModel {
	if st == TypeN {
		return Gapped
	}
	return Linear
}

// TP returns the ST 2110-21 TP media-type parameter value for the sender type
// (§7.1): "2110TPN", "2110TPNL", or "2110TPW".
func (st SenderType) TP() string {
	switch st {
	case TypeN:
		return "2110TPN"
	case TypeNL:
		return "2110TPNL"
	case TypeW:
		return "2110TPW"
	}
	return ""
}

// Params holds the stream parameters needed to evaluate the ST 2110-21 models.
type Params struct {
	// FrameRate is the video frame rate; TFRAME = 1/FrameRate.
	FrameRate media.Rational
	// NPackets is NPACKETS, the number of packets per frame.
	NPackets int64
	// Height is the image height (Media Type Parameter height), used for the
	// gapped progressive TRODEFAULT and interlaced RACTIVE.
	Height int
	// Interlaced indicates interlaced/PsF (affects the gapped schedule).
	Interlaced bool
	// MaxUDP is the UDP datagram size for the VRXFULL minimum term: 1500 for the
	// Standard UDP Size Limit (§7.1), or the Extended limit value otherwise. If
	// zero, 1500 is assumed.
	MaxUDP int
	// RActive overrides the active-to-total time ratio RACTIVE for the gapped
	// model. If zero, the progressive default 1080/1125 is used (§6.3.2).
	RActive media.Rational
	// TotalLines is the total number of raster lines per frame (e.g. 1125), used
	// to derive TLINE = TFRAME/TotalLines for the gapped interlaced schedule
	// (§6.3.3). Only required for interlaced gapped schedules.
	TotalLines int
}

func (p Params) maxUDP() int64 {
	if p.MaxUDP > 0 {
		return int64(p.MaxUDP)
	}
	return 1500
}

// rActive returns the RACTIVE ratio, defaulting to the progressive 1080/1125.
func (p Params) rActive() media.Rational {
	if p.RActive.Num != 0 && p.RActive.Den != 0 {
		return p.RActive
	}
	return media.Rational{Num: 1080, Den: 1125}
}

// TFrameSeconds returns TFRAME in seconds (Den/Num of the frame rate).
func (p Params) TFrameSeconds() float64 {
	return float64(p.FrameRate.Den) / float64(p.FrameRate.Num)
}

// floorDiv returns floor(a/b) for non-negative a and positive b.
func floorDiv(a, b int64) int64 {
	if b == 0 {
		return 0
	}
	return a / b
}

// max64 returns the larger of two int64 values.
func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// VRXFull returns the VRXFULL value (in packets) for the sender type and stream
// parameters (ST 2110-21 §7.1). VRXFULL = MAX(INT(K/MAXUDP), INT(NPACKETS/(D*TFRAME)))
// where (K, D) is (1500*8, 27000) for N/NL and (1500*720, 300) for W.
func (st SenderType) VRXFull(p Params) int64 {
	num, den := p.FrameRate.Num, p.FrameRate.Den
	var minConst, scaleDen int64
	switch st {
	case TypeW:
		minConst, scaleDen = 1500*720, 300
	default: // N, NL
		minConst, scaleDen = 1500*8, 27000
	}
	minTerm := floorDiv(minConst, p.maxUDP())
	// NPACKETS/(scaleDen*TFRAME) = NPACKETS*num/(scaleDen*den).
	scaleTerm := floorDiv(p.NPackets*num, scaleDen*den)
	return max64(minTerm, scaleTerm)
}

// CMax returns the CMAX value (in packets) for the sender type (ST 2110-21
// §7.1): the maximum instantaneous Network Compatibility bucket fullness.
func (st SenderType) CMax(p Params) int64 {
	num, den := p.FrameRate.Num, p.FrameRate.Den
	switch st {
	case TypeN:
		// MAX(4, INT(NPACKETS/(43200*RACTIVE*TFRAME)))
		ra := p.rActive()
		// = NPACKETS*ra.Den*num / (43200*ra.Num*den)
		v := floorDiv(p.NPackets*ra.Den*num, 43200*ra.Num*den)
		return max64(4, v)
	case TypeNL:
		// MAX(4, INT(NPACKETS/(43200*TFRAME)))
		v := floorDiv(p.NPackets*num, 43200*den)
		return max64(4, v)
	case TypeW:
		// MAX(16, INT(NPACKETS/(21600*TFRAME)))
		v := floorDiv(p.NPackets*num, 21600*den)
		return max64(16, v)
	}
	return 0
}

// RNominal returns RNOMINAL, the long-term average packet rate in packets per
// second: NPACKETS / TFRAME (ST 2110-21 §6.6.1).
func (p Params) RNominal() float64 {
	return float64(p.NPackets) / p.TFrameSeconds()
}

// TDrain returns TDRAIN in seconds, the Network Compatibility bucket drain
// interval: (TFRAME/NPACKETS) * (1/β) (ST 2110-21 §6.6.1).
func (p Params) TDrain() float64 {
	return (p.TFrameSeconds() / float64(p.NPackets)) / Beta
}

// TRS returns the Time-Read-Spacing in seconds for the given PRS model
// (ST 2110-21 §6.3.2/§6.4): TFRAME/NPACKETS for linear, scaled by RACTIVE for
// gapped.
func (p Params) TRS(model PRSModel) float64 {
	base := p.TFrameSeconds() / float64(p.NPackets)
	if model == Gapped {
		ra := p.rActive()
		return base * float64(ra.Num) / float64(ra.Den)
	}
	return base
}

// TROffsetDefaultSeconds returns TRODEFAULT in seconds for the gapped progressive
// model (ST 2110-21 §6.3.2): (43/1125)*TFRAME for height ≥ 1080, else
// (28/750)*TFRAME. Interlaced TRODEFAULT depends on the line standard and is not
// computed here.
func (p Params) TROffsetDefaultSeconds() float64 {
	tf := p.TFrameSeconds()
	if p.Height >= 1080 {
		return 43.0 / 1125.0 * tf
	}
	return 28.0 / 750.0 * tf
}
