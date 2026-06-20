package timing21

import "sort"

// TLineSeconds returns TLINE in seconds (TFRAME/TotalLines), or 0 if TotalLines
// is unset.
func (p Params) TLineSeconds() float64 {
	if p.TotalLines <= 0 {
		return 0
	}
	return p.TFrameSeconds() / float64(p.TotalLines)
}

// PacketReadOffset returns the Packet Read Time of packet j relative to the Video
// Transmission Datum TVD (i.e. TPR_j − TVD) in seconds, for the given PRS model
// (ST 2110-21 §6.3/§6.4). For the gapped interlaced model the two-field schedule
// of §6.3.3 is used, which requires Params.TotalLines.
func (p Params) PacketReadOffset(j int64, model PRSModel) float64 {
	trs := p.TRS(model)
	if model == Gapped && p.Interlaced {
		half := p.NPackets / 2
		if j < half {
			return float64(j) * trs
		}
		return p.TFrameSeconds()/2 + p.TLineSeconds()/2 + float64(j-half)*trs
	}
	return float64(j) * trs
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
// emission instants (in seconds, on any timebase whose origin coincides with a
// bucket drain tick). The bucket drains one packet every tDrain seconds when a
// packet is available. Drains at a tick coinciding with an arrival are modeled
// after the arrival, so the returned peak is the worst-case observed value.
func MaxCInst(arrivals []float64, tDrain float64) int64 {
	if len(arrivals) == 0 {
		return 0
	}
	if tDrain <= 0 {
		return int64(len(arrivals))
	}
	a := append([]float64(nil), arrivals...)
	sort.Float64s(a)
	var level, maxLevel int64
	ai := 0
	var k int64
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
