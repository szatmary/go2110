package timing21

import (
	"testing"

	"github.com/szatmary/go2110/sdp"
	"github.com/szatmary/go2110/st2110/media"
)

// 1080p50 example, 3600 packets/frame, Standard UDP (1500).
func params1080p50() Params {
	return Params{
		FrameRate: media.Rational{Num: 50, Den: 1},
		NPackets:  3600,
		Height:    1080,
		MaxUDP:    1500,
	}
}

func TestVRXFull(t *testing.T) {
	p := params1080p50()
	tests := []struct {
		st   SenderType
		want int64
	}{
		{TypeN, 8},   // MAX(8, floor(3600*50/27000)=6) = 8
		{TypeNL, 8},  // same as N
		{TypeW, 720}, // MAX(720, floor(3600*50/300)=600) = 720
	}
	for _, tc := range tests {
		if got := tc.st.VRXFull(p); got != tc.want {
			t.Errorf("VRXFull(%v) = %d, want %d", tc.st, got, tc.want)
		}
	}
}

func TestCMax(t *testing.T) {
	p := params1080p50()
	tests := []struct {
		st   SenderType
		want int64
	}{
		{TypeN, 4},  // MAX(4, floor(3600*1125*50/(43200*1080)) = 4) = 4
		{TypeNL, 4}, // MAX(4, floor(3600*50/43200) = 4) = 4
		{TypeW, 16}, // MAX(16, floor(3600*50/21600) = 8) = 16
	}
	for _, tc := range tests {
		if got := tc.st.CMax(p); got != tc.want {
			t.Errorf("CMax(%v) = %d, want %d", tc.st, got, tc.want)
		}
	}
}

func TestVRXFullScalesWithRate(t *testing.T) {
	// A high packet count pushes the scale term above the minimum for Type N.
	p := Params{FrameRate: media.Rational{Num: 60, Den: 1}, NPackets: 40000, Height: 2160, MaxUDP: 1500}
	// scale = floor(40000*60/27000) = floor(88.88) = 88 > min 8.
	if got := TypeN.VRXFull(p); got != 88 {
		t.Errorf("VRXFull = %d, want 88", got)
	}
}

func TestTPAndPRS(t *testing.T) {
	if TypeN.TP() != "2110TPN" || TypeNL.TP() != "2110TPNL" || TypeW.TP() != "2110TPW" {
		t.Error("TP strings wrong")
	}
	if TypeN.PRS() != Gapped || TypeNL.PRS() != Linear || TypeW.PRS() != Linear {
		t.Error("PRS mapping wrong")
	}
}

func TestTimingValues(t *testing.T) {
	p := params1080p50()
	if tf := p.TFrameSeconds(); tf != 0.02 {
		t.Errorf("TFRAME = %v, want 0.02", tf)
	}
	if rn := p.RNominal(); rn != 180000 {
		t.Errorf("RNOMINAL = %v, want 180000", rn)
	}
	// TDRAIN = (0.02/3600)/1.10
	want := (0.02 / 3600) / 1.10
	if td := p.TDrain(); td != want {
		t.Errorf("TDRAIN = %v, want %v", td, want)
	}
	// Gapped TRS is shorter than linear TRS by the factor RACTIVE.
	if p.TRS(Gapped) >= p.TRS(Linear) {
		t.Error("gapped TRS should be < linear TRS")
	}
	// TRODEFAULT progressive >=1080: (43/1125)*TFRAME.
	if got, want := p.TROffsetDefaultSeconds(), 43.0/1125.0*0.02; got != want {
		t.Errorf("TRODEFAULT = %v, want %v", got, want)
	}
}

func TestMaxCInst(t *testing.T) {
	// A burst of 3 packets at t=0: peak fullness is 3 (drain hasn't run yet).
	if got := MaxCInst([]float64{0, 0, 0}, 1); got != 3 {
		t.Errorf("burst peak = %d, want 3", got)
	}
	// Evenly spaced at exactly the drain interval: steady state of 1.
	if got := MaxCInst([]float64{0, 1, 2, 3}, 1); got != 1 {
		t.Errorf("steady peak = %d, want 1", got)
	}
	// Faster than drain: backlog grows.
	if got := MaxCInst([]float64{0, 0.4, 0.8, 1.2}, 1); got < 2 {
		t.Errorf("fast peak = %d, want >=2", got)
	}
}

func TestVRX(t *testing.T) {
	peak, under := VRX([]float64{0, 1, 2}, []float64{0, 1, 2})
	if under || peak != 1 {
		t.Errorf("matched: peak=%d under=%v", peak, under)
	}
	peak, under = VRX([]float64{0, 0, 0}, []float64{0, 1, 2})
	if under || peak != 3 {
		t.Errorf("burst: peak=%d under=%v", peak, under)
	}
	_, under = VRX([]float64{5}, []float64{0})
	if !under {
		t.Error("expected underflow when emit after read")
	}
}

func TestComplianceHelpers(t *testing.T) {
	p := params1080p50()
	read := p.ReadSchedule(p.NPackets, Linear)
	// A sender emitting exactly on the read schedule is VRX-compliant (peak 1).
	if !TypeNL.VRXCompliant(p, read, read) {
		t.Error("on-schedule sender should be VRX compliant")
	}
	// Emitting all packets in a single instant overflows the VRX buffer.
	burst := make([]float64, p.NPackets)
	if TypeNL.VRXCompliant(p, burst, read) {
		t.Error("burst sender should overflow VRX")
	}
}

func TestSDPSenderType(t *testing.T) {
	fp := sdp.FormatParameters{Format: "112"}
	SetTP(&fp, TypeW)
	SetTROFF(&fp, 0)
	st, ok := ParseSenderType(fp)
	if !ok || st != TypeW {
		t.Errorf("ParseSenderType = %v %v", st, ok)
	}
	if v, _ := fp.Get("TP"); v != "2110TPW" {
		t.Errorf("TP = %q", v)
	}
}
