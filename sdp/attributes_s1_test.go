package sdp

import "testing"

// TestS1_RefClockDomainNmbr pins the RFC 7273 "domain-nmbr=" PTP domain form
// (used by ST 2110-10:2022) alongside the legacy bare form and the no-domain
// form, all round-tripping.
func TestS1_RefClockDomainNmbr(t *testing.T) {
	cases := []struct {
		in   string
		want RefClock
	}{
		{"ptp=IEEE1588-2008:39-A7-94-FF-FE-07-CB-D0:domain-nmbr=127",
			RefClock{PTP: true, PTPProfile: "IEEE1588-2008", GMID: "39-A7-94-FF-FE-07-CB-D0", Domain: 127, DomainNmbr: true}},
		{"ptp=IEEE1588-2008:39-A7-94-FF-FE-07-CB-D0:37",
			RefClock{PTP: true, PTPProfile: "IEEE1588-2008", GMID: "39-A7-94-FF-FE-07-CB-D0", Domain: 37}},
		{"ptp=IEEE802.1AS-2011:39-A7-94-FF-FE-07-CB-D0",
			RefClock{PTP: true, PTPProfile: "IEEE802.1AS-2011", GMID: "39-A7-94-FF-FE-07-CB-D0", NoDomain: true}},
	}
	for _, c := range cases {
		got, err := ParseRefClock(c.in)
		if err != nil {
			t.Errorf("ParseRefClock(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseRefClock(%q) = %+v, want %+v", c.in, got, c.want)
		}
		if rt := got.String(); rt != c.in {
			t.Errorf("RefClock.String() = %q, want %q", rt, c.in)
		}
	}
}

// TestS1_RefClockRejectsBadEUI pins RFC 7273 Figure 1: an EUI-48/64 that is not
// the right number of 2-hex-digit hyphenated octets is rejected.
func TestS1_RefClockRejectsBadEUI(t *testing.T) {
	bad := []string{
		"ptp=IEEE1588-2008:39-A7-94-FF-FE-07-CB:37", // only 7 octets (EUI-64 needs 8)
		"ptp=IEEE1588-2008:39-A7-94-FF-FE-07-CB-XY:0", // non-hex octet
		"localmac=7C-E9-D3-1B-9A",                     // 5 octets (EUI-48 needs 6)
		"localmac=7C-E9-D3-1B-9A-AF-00",               // 7 octets
	}
	for _, in := range bad {
		if _, err := ParseRefClock(in); err != ErrBadEUI {
			t.Errorf("ParseRefClock(%q) = %v, want ErrBadEUI", in, err)
		}
	}
	// A valid EUI-48 localmac still parses.
	if _, err := ParseRefClock("localmac=7C-E9-D3-1B-9A-AF"); err != nil {
		t.Errorf("valid localmac: %v", err)
	}
}

// TestS1_MediaClockRateAndID pins the RFC 7273 §5.2 "rate=N/D" modifier and the
// §5.3 "id=" master-clock prefix.
func TestS1_MediaClockRateAndID(t *testing.T) {
	cases := []struct {
		in   string
		want MediaClock
	}{
		{"direct=0 rate=48000/1", MediaClock{Direct: true, Offset: 0, RateNum: 48000, RateDen: 1}},
		{"direct=963214424 rate=24000/1001", MediaClock{Direct: true, Offset: 963214424, RateNum: 24000, RateDen: 1001}},
		{"id=src:word1 sender", MediaClock{Sender: true, ID: "src:word1"}},
		{"id=stream7 direct=0", MediaClock{Direct: true, Offset: 0, ID: "stream7"}},
		{"sender", MediaClock{Sender: true}},
		{"direct=0", MediaClock{Direct: true, Offset: 0}},
	}
	for _, c := range cases {
		got, err := ParseMediaClock(c.in)
		if err != nil {
			t.Errorf("ParseMediaClock(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseMediaClock(%q) = %+v, want %+v", c.in, got, c.want)
		}
		if rt := got.String(); rt != c.in {
			t.Errorf("MediaClock.String() = %q, want %q", rt, c.in)
		}
	}
}
