package video

import (
	"testing"

	"github.com/szatmary/go2110/sdp"
	"github.com/szatmary/go2110/st2110/media"
)

func TestFMTPExample(t *testing.T) {
	// ST 2110-20 §7.7 informative example fmtp value.
	f := Format{
		Sampling:       SamplingYCbCr422,
		Width:          1280,
		Height:         720,
		ExactFrameRate: media.Rational{Num: 60000, Den: 1001},
		Depth:          Depth10,
		TCS:            "SDR",
		Colorimetry:    "BT709",
		PackingMode:    PackingGPM,
		SSN:            "ST2110-20:2017",
	}
	want := "112 sampling=YCbCr-4:2:2; width=1280; height=720; exactframerate=60000/1001; depth=10; TCS=SDR; colorimetry=BT709; PM=2110GPM; SSN=ST2110-20:2017"
	got := f.FMTP(112).String()
	if got != want {
		t.Fatalf("FMTP =\n %q\nwant\n %q", got, want)
	}
}

func TestFMTPRoundTrip(t *testing.T) {
	f := Format{
		Sampling:       SamplingYCbCr422,
		Width:          1920,
		Height:         1080,
		ExactFrameRate: media.Rational{Num: 30000, Den: 1001},
		Depth:          Depth10,
		TCS:            "PQ",
		Colorimetry:    "BT2100",
		Range:          "NARROW",
		PackingMode:    PackingBPM,
		SSN:            "ST2110-20:2022",
		Interlaced:     true,
		MaxUDP:         8960,
		PAR:            "12:11",
	}
	fp := f.FMTP(96)
	got, err := ParseFMTP(fp)
	if err != nil {
		t.Fatal(err)
	}
	if got != f {
		t.Fatalf("round-trip:\n got %+v\nwant %+v", got, f)
	}
}

func TestParseFMTPFromSDPExample(t *testing.T) {
	raw := "a=fmtp:112 sampling=YCbCr-4:2:2; width=1280; height=720; exactframerate=60000/1001; depth=10; TCS=SDR; colorimetry=BT709; PM=2110GPM; SSN=ST2110-20:2017"
	attr := raw[len("a="):]
	key, val, _ := splitColon(attr)
	if key != "fmtp" {
		t.Fatalf("key = %q", key)
	}
	fp := sdp.ParseFormatParameters(val)
	f, err := ParseFMTP(fp)
	if err != nil {
		t.Fatal(err)
	}
	if f.Sampling != SamplingYCbCr422 || f.Width != 1280 || f.Height != 720 || f.Depth != Depth10 {
		t.Fatalf("parsed = %+v", f)
	}
	if f.ExactFrameRate != (media.Rational{Num: 60000, Den: 1001}) {
		t.Fatalf("rate = %+v", f.ExactFrameRate)
	}
}

func splitColon(s string) (string, string, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			return s[:i], s[i+1:], true
		}
	}
	return s, "", false
}

func TestRTPMapHelper(t *testing.T) {
	r := RTPMap(112)
	if r.String() != "112 raw/90000" {
		t.Fatalf("rtpmap = %q", r.String())
	}
}
