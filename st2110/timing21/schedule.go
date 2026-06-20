package timing21

import (
	"math"
	"sort"
)

// TLineSeconds returns TLINE in seconds (TFRAME/TotalLines), or 0 if TotalLines
// is unset.
func (p Params) TLineSeconds() float64 {
	if p.TotalLines <= 0 {
		return 0
	}
	return p.TFrameSeconds() / float64(p.TotalLines)
}

// PacketReadOffset returns the Packet Read Time of packet j relative to the frame
// alignment point in seconds, for the given PRS model (ST 2110-21 §6.3/§6.4):
// TROFFSET + (j × TRS) for the linear and gapped-progressive models, with the
// two-field offset of §6.3.3 added for the gapped interlaced model. The constant
// TROFFSET (Params.TROffset) shifts the whole schedule; the standard default is
// TROffsetDefaultSeconds. The gapped interlaced schedule requires Params.TotalLines.
func (p Params) PacketReadOffset(j int64, model PRSModel) float64 {
	trs := p.TRS(model)
	if model == Gapped && p.Interlaced {
		half := p.NPackets / 2
		if j < half {
			return p.TROffset + float64(j)*trs
		}
		return p.TROffset + p.TFrameSeconds()/2 + p.TLineSeconds()/2 + float64(j-half)*trs
	}
	return p.TROffset + float64(j)*trs
}

// ReadSchedule returns the Packet Read Time offsets (relative to TVD) for packets
// 0..n-1 under the given PRS model.
func (p Params) ReadSchedule(n int64, model PRSModel) []float64 {
	out := make([]float64, n)
	for j := int64(0); j < n; j++ {
		out[j] = p.PacketReadOffset(j, model)
	}
	return out
}

// MaxCInst simulates the Network Compatibility leaky bucket (ST 2110-21 §6.6.1)
// and returns the peak instantaneous fullness CINST given the actual packet
// emission instants in seconds on an epoch-absolute timebase (the same timebase
// as the media clock; t = 0 is the SMPTE Epoch). The bucket drains one packet
// every tDrain seconds when a packet is available, on a fixed grid of drain ticks
// at integer multiples of tDrain since the Epoch (N × TDRAIN) — the grid is
// anchored to the Epoch, not to the first packet, so the phase of emissions
// relative to that grid is what the model measures. Drains at a tick coinciding
// with an arrival are modeled after the arrival, so the returned peak is the
// worst-case observed value.
func MaxCInst(arrivals []float64, tDrain float64) int64 {
	if len(arrivals) == 0 {
		return 0
	}
	if tDrain <= 0 {
		return int64(len(arrivals))
	}
	a := append([]float64(nil), arrivals...)
	sort.Float64s(a)
	// Begin the drain counter at the last epoch-grid tick at or before the first
	// arrival; earlier ticks drain an empty bucket and cannot affect the peak.
	// Computing k0 directly (rather than iterating from 0) keeps the simulation
	// O(packets + drains-during-stream) even for a large epoch-absolute origin.
	k := int64(math.Floor(a[0] / tDrain))
	var level, maxLevel int64
	ai := 0
	for ai < len(a) {
		tickTime := float64(k) * tDrain
		if tickTime < a[ai] { // drain occurs strictly before the next arrival
			if level > 0 {
				level--
			}
			k++
			continue
		}
		level++ // arrival (also when an arrival coincides with a tick)
		if level > maxLevel {
			maxLevel = level
		}
		ai++
	}
	return maxLevel
}

// NetworkCompatible reports whether the emission instants satisfy the sender
// type's Network Compatibility Model: CINST never exceeds CMAX (ST 2110-21
// §6.6.1, §7.1).
func (st SenderType) NetworkCompatible(p Params, emissions []float64) bool {
	return MaxCInst(emissions, p.TDrain()) <= st.CMax(p)
}

// VRX evaluates the Virtual Receiver Buffer model (ST 2110-21 §6.6.2) for one
// frame. emit[j] is the actual emission instant of packet j and read[j] its
// Packet Read Time (both on the same timebase, same length). It returns the peak
// bucket occupancy and whether the bucket underflows (some packet is scheduled
// to be read before it is emitted, i.e. emit[j] > read[j]).
func VRX(emit, read []float64) (peak int64, underflow bool) {
	n := len(emit)
	if len(read) < n {
		n = len(read)
	}
	for j := 0; j < n; j++ {
		if emit[j] > read[j] {
			underflow = true
		}
	}
	// Peak occupancy = max over time of (#emit ≤ t) − (#read ≤ t); process
	// emissions before reads at an equal instant to capture the peak.
	type ev struct {
		t  float64
		in bool
	}
	evs := make([]ev, 0, 2*n)
	for j := 0; j < n; j++ {
		evs = append(evs, ev{emit[j], true}, ev{read[j], false})
	}
	sort.SliceStable(evs, func(i, j int) bool {
		if evs[i].t != evs[j].t {
			return evs[i].t < evs[j].t
		}
		return evs[i].in && !evs[j].in // arrivals first at equal time
	})
	var level int64
	for _, e := range evs {
		if e.in {
			level++
			if level > peak {
				peak = level
			}
		} else if level > 0 {
			level--
		}
	}
	return peak, underflow
}

// VRXCompliant reports whether the emission instants comply with the sender
// type's Virtual Receiver Buffer model for a frame: the buffer neither overflows
// (peak ≤ VRXFULL) nor underflows. read holds the Packet Read Times (e.g. from
// ReadSchedule) and emit the actual emission instants, on the same timebase.
func (st SenderType) VRXCompliant(p Params, emit, read []float64) bool {
	peak, underflow := VRX(emit, read)
	return !underflow && peak <= st.VRXFull(p)
}

// ComplianceReport summarizes an ST 2110-21 sender compliance evaluation for one
// frame: the Network Compatibility leaky-bucket result (§6.6.1) and the Virtual
// Receiver Buffer result (§6.6.2).
type ComplianceReport struct {
	CInst     int64 // peak instantaneous Network Compatibility bucket fullness
	CMax      int64 // permitted CMAX for the sender type
	VRXPeak   int64 // peak Virtual Receiver Buffer occupancy
	VRXFull   int64 // permitted VRXFULL for the sender type
	Underflow bool  // a packet was scheduled to be read before it was emitted
	NetworkOK bool  // CInst ≤ CMax
	VRXOK     bool  // not Underflow and VRXPeak ≤ VRXFull
}

// Compliant reports overall ST 2110-21 conformance for the sender type.
func (r ComplianceReport) Compliant() bool { return r.NetworkOK && r.VRXOK }

// Validate evaluates both ST 2110-21 sender models for one frame and ties
// together VRXFull/CMax/MaxCInst/VRX into a single report. emit holds the actual
// packet emission instants and read the Packet Read Times (e.g. from
// ReadSchedule), both in seconds on a shared epoch-absolute timebase. It is the
// public entry point for sender traffic-shaping conformance checking.
func (st SenderType) Validate(p Params, emit, read []float64) ComplianceReport {
	cinst := MaxCInst(emit, p.TDrain())
	cmax := st.CMax(p)
	peak, under := VRX(emit, read)
	full := st.VRXFull(p)
	return ComplianceReport{
		CInst:     cinst,
		CMax:      cmax,
		VRXPeak:   peak,
		VRXFull:   full,
		Underflow: under,
		NetworkOK: cinst <= cmax,
		VRXOK:     !under && peak <= full,
	}
}
