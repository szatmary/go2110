package anc

import (
	"reflect"
	"testing"

	"github.com/szatmary/go2110/st2110/media"
)

func TestANCRTPMap(t *testing.T) {
	if got := RTPMap(100).String(); got != "100 smpte291/90000" {
		t.Fatalf("rtpmap = %q", got)
	}
}

func TestDIDSDID(t *testing.T) {
	d := DIDSDID{DID: 0x61, SDID: 0x02}
	if d.String() != "{0x61,0x02}" {
		t.Fatalf("DIDSDID = %q", d.String())
	}
	got, err := ParseDIDSDID("{0x61,0x02}")
	if err != nil || got != d {
		t.Fatalf("ParseDIDSDID = %+v err=%v", got, err)
	}
}

func TestANCFMTPRoundTrip(t *testing.T) {
	f := Format{
		ExactFrameRate: media.Rational{Num: 30000, Den: 1001},
		DIDSDIDs:       []DIDSDID{{0x61, 0x02}, {0x41, 0x05}},
		VPIDCode:       132,
		TM:             "CTM",
		SSN:            "ST2110-40:2018",
	}
	fp := f.FMTP(100)
	got, err := ParseFMTP(fp)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, f) {
		t.Fatalf("round-trip:\n got %+v\nwant %+v", got, f)
	}
	// The fmtp string should carry both DID_SDID entries.
	s := fp.String()
	if want := "DID_SDID={0x61,0x02}"; !contains(s, want) {
		t.Errorf("fmtp %q missing %q", s, want)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
