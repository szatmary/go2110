package cv22

import (
	"testing"

	"github.com/szatmary/go2110/sdp"
	"github.com/szatmary/go2110/st2110/media"
	"github.com/szatmary/go2110/st2110/timing21"
)

// TestC1_FractionalFrameRateRoundTrip pins the ST 2110-22 §7.4 exactframerate
// signalling: a non-integer rate (60000/1001) must survive emit->parse. The
// buggy code emitted a decimal a=framerate that its own exactframerate parser
// rejected, silently yielding FrameRate 0/0.
func TestC1_FractionalFrameRateRoundTrip(t *testing.T) {
	f := Format{
		Subtype:    "jxsv",
		Width:      1920,
		Height:     1080,
		FrameRate:  media.Rational{Num: 60000, Den: 1001},
		SenderType: timing21.TypeN,
	}
	m := f.MediaDescription(5004, 112)

	// The authoritative rate is carried in fmtp as exactframerate=60000/1001.
	fp, _ := m.FormatParameters()
	if v, ok := fp.Get("exactframerate"); !ok || v != "60000/1001" {
		t.Fatalf("exactframerate fmtp param = %q ok=%v, want 60000/1001", v, ok)
	}

	got, err := ParseMediaDescription(m)
	if err != nil {
		t.Fatal(err)
	}
	want := media.Rational{Num: 60000, Den: 1001}
	if got.FrameRate != want {
		t.Fatalf("round-trip FrameRate = %v, want %v (buggy code lost it as 0/0)", got.FrameRate, want)
	}
}

// TestC1_MalformedExactFrameRateErrors pins that a malformed exactframerate is
// surfaced as an error rather than swallowed.
func TestC1_MalformedExactFrameRateErrors(t *testing.T) {
	m := &sdp.MediaDescription{
		Media: "video", Port: 5004, Protocol: "RTP/AVP", Formats: []string{"112"},
	}
	m.SetAttribute("rtpmap", "112 jxsv/90000")
	m.SetAttribute("fmtp", "112 width=1920; height=1080; exactframerate=29.97; TP=2110TPN")
	if _, err := ParseMediaDescription(m); err == nil {
		t.Fatal("expected error for malformed exactframerate '29.97', got nil")
	}
}
