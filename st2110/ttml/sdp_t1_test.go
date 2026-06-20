package ttml

import "testing"

// TestT1_SDPCodecs pins RFC 8759 §11.2.1 Figure 5: the mandatory a=fmtp line
// "charset=utf-8;codecs=<profile>". The buggy SDP emitted no fmtp at all, so the
// MUST "codecs" parameter (§6.1.3/§11.2) was absent.
func TestT1_SDPCodecs(t *testing.T) {
	m := MediaDescription(30000, 112, "")
	v, ok := m.GetAttribute("fmtp")
	if !ok {
		t.Fatal("fmtp attribute missing; codecs= is a MUST (RFC 8759 §11.2)")
	}
	// Exact string from RFC 8759 Figure 5 (default im2t profile).
	if v != "112 charset=utf-8;codecs=im2t" {
		t.Fatalf("fmtp = %q, want %q", v, "112 charset=utf-8;codecs=im2t")
	}
	// An explicit profile must be honoured.
	if v, _ := MediaDescription(30000, 112, "im1t").GetAttribute("fmtp"); v != "112 charset=utf-8;codecs=im1t" {
		t.Fatalf("explicit profile fmtp = %q", v)
	}
}
