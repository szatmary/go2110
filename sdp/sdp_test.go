package sdp

import (
	"reflect"
	"strings"
	"testing"
)

// annexB is the ST 2110-10:2022 Annex B example SDP (a redundant 720p59.94
// stream), reproduced as a single logical line per fmtp (the PDF wraps it).
// Line endings are CRLF per RFC 4566.
var annexBLines = []string{
	"v=0",
	"o=- 123456 11 IN IP4 192.168.100.2",
	"s=Example of a SMPTE ST2110-20 signal",
	"i=this example is for 720p video at 59.94",
	"t=0 0",
	"a=recvonly",
	"a=group:DUP primary secondary",
	"m=video 50000 RTP/AVP 112",
	"c=IN IP4 239.100.9.10/32",
	"a=source-filter: incl IN IP4 239.100.9.10 192.168.100.2",
	"a=rtpmap:112 raw/90000",
	"a=fmtp:112 sampling=YCbCr-4:2:2; width=1280; height=720; exactframerate=60000/1001; depth=10; TCS=SDR; colorimetry=BT709; PM=2110GPM; SSN=ST2110-20:2017; TSMODE=SAMP; TSDELAY=0",
	"a=ts-refclk:ptp=IEEE1588-2008:39-A7-94-FF-FE-07-CB-D0:37",
	"a=mediaclk:direct=0",
	"a=mid:primary",
	"m=video 50020 RTP/AVP 112",
	"c=IN IP4 239.101.9.10/32",
	"a=source-filter: incl IN IP4 239.101.9.10 192.168.101.2",
	"a=rtpmap:112 raw/90000",
	"a=fmtp:112 sampling=YCbCr-4:2:2; width=1280; height=720; exactframerate=60000/1001; depth=10; TCS=SDR; colorimetry=BT709; PM=2110GPM; SSN=ST2110-20:2017; TSMODE=SAMP; TSDELAY=0",
	"a=ts-refclk:ptp=IEEE1588-2008:39-A7-94-FF-FE-07-CB-D0:37",
	"a=mediaclk:direct=0",
	"a=mid:secondary",
}

func annexB() string { return strings.Join(annexBLines, "\r\n") + "\r\n" }

func TestParseAnnexB(t *testing.T) {
	s, err := Unmarshal(annexB())
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if s.Version != 0 {
		t.Errorf("version = %d", s.Version)
	}
	if s.Origin.UnicastAddress != "192.168.100.2" || s.Origin.SessionID != "123456" {
		t.Errorf("origin = %+v", s.Origin)
	}
	if len(s.Timing) != 1 || s.Timing[0] != (Timing{0, 0}) {
		t.Errorf("timing = %+v", s.Timing)
	}
	if !hasFlag(s.Attributes, "recvonly") {
		t.Error("missing a=recvonly")
	}
	gv, ok := s.GetAttribute("group")
	if !ok {
		t.Fatal("missing group")
	}
	g, err := ParseGroup(gv)
	if err != nil || g.Semantics != "DUP" || !reflect.DeepEqual(g.MIDs, []string{"primary", "secondary"}) {
		t.Errorf("group = %+v err=%v", g, err)
	}
	if len(s.MediaDescriptions) != 2 {
		t.Fatalf("got %d media sections", len(s.MediaDescriptions))
	}

	m := s.MediaDescriptions[0]
	if m.Media != "video" || m.Port != 50000 || m.Protocol != "RTP/AVP" || !reflect.DeepEqual(m.Formats, []string{"112"}) {
		t.Errorf("m= = %+v", m)
	}
	if m.Connection == nil || m.Connection.Address != "239.100.9.10/32" {
		t.Errorf("connection = %+v", m.Connection)
	}
	rm, ok := m.RTPMap()
	if !ok || rm.PayloadType != 112 || rm.EncodingName != "raw" || rm.ClockRate != 90000 {
		t.Errorf("rtpmap = %+v ok=%v", rm, ok)
	}
	rc, ok := m.RefClock()
	if !ok || !rc.PTP || rc.GMID != "39-A7-94-FF-FE-07-CB-D0" || rc.Domain != 37 {
		t.Errorf("ts-refclk = %+v ok=%v", rc, ok)
	}
	mc, ok := m.MediaClock()
	if !ok || !mc.Direct || mc.Offset != 0 {
		t.Errorf("mediaclk = %+v ok=%v", mc, ok)
	}
	mid, _ := m.MID()
	if mid != "primary" {
		t.Errorf("mid = %q", mid)
	}

	fp, ok := m.FormatParameters()
	if !ok {
		t.Fatal("missing fmtp")
	}
	if fp.Format != "112" {
		t.Errorf("fmtp format = %q", fp.Format)
	}
	for k, want := range map[string]string{
		"sampling": "YCbCr-4:2:2", "width": "1280", "height": "720",
		"exactframerate": "60000/1001", "depth": "10", "TCS": "SDR",
		"colorimetry": "BT709", "PM": "2110GPM", "SSN": "ST2110-20:2017",
		"TSMODE": "SAMP", "TSDELAY": "0",
	} {
		if got, ok := fp.Get(k); !ok || got != want {
			t.Errorf("fmtp %s = %q ok=%v, want %q", k, got, ok, want)
		}
	}

	sf, ok := m.GetAttribute("source-filter")
	if !ok {
		t.Fatal("missing source-filter")
	}
	pf, err := ParseSourceFilter(sf)
	if err != nil || pf.Mode != "incl" || pf.DestAddress != "239.100.9.10" || !reflect.DeepEqual(pf.Sources, []string{"192.168.100.2"}) {
		t.Errorf("source-filter = %+v err=%v", pf, err)
	}
}

func TestBuildAnnexBRoundTrip(t *testing.T) {
	// Parse, re-marshal, and re-parse: the second parse must equal the first.
	s1, err := Unmarshal(annexB())
	if err != nil {
		t.Fatal(err)
	}
	out := s1.Marshal()
	s2, err := Unmarshal(out)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if !reflect.DeepEqual(s1, s2) {
		t.Errorf("round-trip mismatch:\n%s", out)
	}
	// Marshaled output must reproduce the canonical Annex B text exactly.
	if out != annexB() {
		t.Errorf("marshal mismatch:\n got:\n%q\nwant:\n%q", out, annexB())
	}
}

func TestRefClockForms(t *testing.T) {
	tests := []struct {
		in   string
		want RefClock
	}{
		{"ptp=IEEE1588-2008:39-A7-94-FF-FE-07-CB-D0:37", RefClock{PTP: true, PTPProfile: "IEEE1588-2008", GMID: "39-A7-94-FF-FE-07-CB-D0", Domain: 37}},
		{"ptp=IEEE1588-2008:traceable", RefClock{PTP: true, PTPProfile: "IEEE1588-2008", Traceable: true}},
		{"localmac=7C-E9-D3-1B-9A-AF", RefClock{LocalMAC: "7C-E9-D3-1B-9A-AF"}},
	}
	for _, tc := range tests {
		got, err := ParseRefClock(tc.in)
		if err != nil {
			t.Errorf("ParseRefClock(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseRefClock(%q) = %+v, want %+v", tc.in, got, tc.want)
		}
		if rt := got.String(); rt != tc.in {
			t.Errorf("RefClock.String() = %q, want %q", rt, tc.in)
		}
	}
}

func TestMediaClockForms(t *testing.T) {
	for _, in := range []string{"direct=0", "direct=12345", "sender"} {
		mc, err := ParseMediaClock(in)
		if err != nil {
			t.Errorf("ParseMediaClock(%q): %v", in, err)
			continue
		}
		if mc.String() != in {
			t.Errorf("round-trip %q -> %q", in, mc.String())
		}
	}
}

func TestFmtpRoundTrip(t *testing.T) {
	in := "112 sampling=YCbCr-4:2:2; width=1280; interlace; depth=10"
	fp := ParseFormatParameters(in)
	if !fp.Has("interlace") {
		t.Error("interlace flag not parsed")
	}
	if _, ok := fp.Get("interlace"); ok {
		t.Error("interlace should have no value")
	}
	if fp.String() != in {
		t.Errorf("round-trip: %q -> %q", in, fp.String())
	}
}
