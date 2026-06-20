package cv22

import (
	"testing"

	"github.com/szatmary/go2110/sdp"
	"github.com/szatmary/go2110/st2110/media"
	"github.com/szatmary/go2110/st2110/timing21"
)

func TestExampleMediaSection(t *testing.T) {
	// Reproduce the media-level lines of the ST 2110-22 §7.5 example.
	f := Format{
		Subtype:     "vc2",
		Width:       1920,
		Height:      1080,
		FrameRate:   media.Rational{Num: 25, Den: 1},
		SenderType:  timing21.TypeW,
		SSN:         "ST2110-22:2022",
		BitrateKbps: 216000,
		Extra:       []sdp.Param{{Name: "profile", Value: "HQ", HasValue: true}},
	}
	m := f.MediaDescription(5004, 96)

	if m.Media != "video" || m.Port != 5004 {
		t.Fatalf("m= = %+v", m)
	}
	if len(m.Bandwidth) != 1 || m.Bandwidth[0].Type != "AS" || m.Bandwidth[0].Value != 216000 {
		t.Errorf("bandwidth = %+v", m.Bandwidth)
	}
	if v, _ := m.GetAttribute("rtpmap"); v != "96 vc2/90000" {
		t.Errorf("rtpmap = %q", v)
	}
	if v, _ := m.GetAttribute("framerate"); v != "25" {
		t.Errorf("framerate = %q", v)
	}
	want := "96 profile=HQ; width=1920; height=1080; TP=2110TPW; SSN=ST2110-22:2022"
	if v, _ := m.GetAttribute("fmtp"); v != want {
		t.Errorf("fmtp =\n %q\nwant\n %q", v, want)
	}
}

func TestRoundTrip(t *testing.T) {
	f := Format{
		Subtype:     "jxsv",
		Width:       3840,
		Height:      2160,
		FrameRate:   media.Rational{Num: 60000, Den: 1001},
		SenderType:  timing21.TypeN,
		CMax:        4,
		SSN:         "ST2110-22:2022",
		BitrateKbps: 1000000,
		Extra: []sdp.Param{
			{Name: "packetmode", Value: "0", HasValue: true},
			{Name: "profile", Value: "High444.12", HasValue: true},
		},
	}
	m := f.MediaDescription(5004, 112)
	got, err := ParseMediaDescription(m)
	if err != nil {
		t.Fatal(err)
	}
	if got.Subtype != f.Subtype || got.Width != f.Width || got.Height != f.Height ||
		got.SenderType != f.SenderType || got.CMax != f.CMax || got.SSN != f.SSN ||
		got.BitrateKbps != f.BitrateKbps {
		t.Fatalf("round-trip core mismatch:\n got %+v\nwant %+v", got, f)
	}
	if len(got.Extra) != 2 || got.Extra[0].Name != "packetmode" || got.Extra[1].Name != "profile" {
		t.Errorf("extra params = %+v", got.Extra)
	}
}

func TestBitrateCalc(t *testing.T) {
	// 100000 octets/frame at 25 fps = 100000*8*25 = 20,000,000 bits/s = 20000 kbps.
	if got := BitrateKbpsFor(100000, media.Rational{Num: 25, Den: 1}); got != 20000 {
		t.Errorf("bitrate = %d, want 20000", got)
	}
	// Non-integer frame rate rounds up.
	got := BitrateKbpsFor(125000, media.Rational{Num: 30000, Den: 1001})
	// 125000*8*30000/1001/1000 = 29970.03 -> ceil 29971
	if got != 29971 {
		t.Errorf("bitrate = %d, want 29971", got)
	}
}
