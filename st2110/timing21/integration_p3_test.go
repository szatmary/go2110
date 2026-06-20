package timing21

import (
	"testing"

	"github.com/szatmary/go2110/st2110/media"
	"github.com/szatmary/go2110/st2110/video"
)

// TestP3_PacketizerStreamIsCompliant is the integration test for the ST 2110-21
// validators (previously dead code with zero callers). It packetizes a real
// ST 2110-20 frame, has the Narrow sender emit exactly on its gapped Packet Read
// Schedule, and checks that the emission stream passes both sender models through
// the public Validate entry point (and the NetworkCompatible/VRXCompliant helpers).
func TestP3_PacketizerStreamIsCompliant(t *testing.T) {
	f := video.Format{
		Sampling:       video.SamplingYCbCr422,
		Depth:          video.Depth10,
		Width:          1920,
		Height:         1080,
		ExactFrameRate: media.Rational{Num: 50, Den: 1},
	}
	fr := video.NewFrame(f)
	pf, err := fr.Pack()
	if err != nil {
		t.Fatal(err)
	}
	pkts, err := pf.Packetize(video.PacketizeOptions{PayloadType: 96})
	if err != nil {
		t.Fatal(err)
	}
	if len(pkts) == 0 {
		t.Fatal("no packets produced")
	}

	p := Params{
		FrameRate: f.ExactFrameRate,
		NPackets:  int64(len(pkts)),
		Height:    f.Height,
		MaxUDP:    1500,
	}
	// A compliant Narrow sender emits exactly on its gapped Packet Read Schedule.
	read := p.ReadSchedule(p.NPackets, TypeN.PRS())
	emit := read

	rep := TypeN.Validate(p, emit, read)
	if !rep.Compliant() {
		t.Fatalf("Narrow on-schedule stream not compliant: %+v", rep)
	}
	if rep.CInst > rep.CMax {
		t.Errorf("CInst %d exceeds CMax %d", rep.CInst, rep.CMax)
	}
	if rep.VRXPeak > rep.VRXFull {
		t.Errorf("VRXPeak %d exceeds VRXFull %d", rep.VRXPeak, rep.VRXFull)
	}
	if !TypeN.NetworkCompatible(p, emit) {
		t.Error("NetworkCompatible returned false for an on-schedule Narrow sender")
	}
	if !TypeN.VRXCompliant(p, emit, read) {
		t.Error("VRXCompliant returned false for an on-schedule Narrow sender")
	}
}
