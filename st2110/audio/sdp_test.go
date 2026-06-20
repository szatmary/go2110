package audio

import (
	"reflect"
	"testing"
)

func TestChannelOrderParse(t *testing.T) {
	tests := []struct {
		in     string
		groups []string
		total  int
	}{
		{"SMPTE2110.(51,ST)", []string{"51", "ST"}, 8},
		{"SMPTE2110.(M,M,M,M,ST,U02)", []string{"M", "M", "M", "M", "ST", "U02"}, 8},
		{"SMPTE2110.(222)", []string{"222"}, 24},
		{"SMPTE2110.(SGRP,SGRP)", []string{"SGRP", "SGRP"}, 8},
	}
	for _, tc := range tests {
		co, err := ParseChannelOrder(tc.in)
		if err != nil {
			t.Errorf("ParseChannelOrder(%q): %v", tc.in, err)
			continue
		}
		if co.Convention != "SMPTE2110" || !reflect.DeepEqual(co.Groups, tc.groups) {
			t.Errorf("parsed %q = %+v", tc.in, co)
		}
		total, err := co.TotalChannels()
		if err != nil || total != tc.total {
			t.Errorf("%q total = %d (err %v), want %d", tc.in, total, err, tc.total)
		}
		if co.String() != tc.in {
			t.Errorf("round-trip %q -> %q", tc.in, co.String())
		}
	}
}

func TestGroupChannelCount(t *testing.T) {
	if n, ok := GroupChannelCount("U05"); !ok || n != 5 {
		t.Errorf("U05 = %d %v", n, ok)
	}
	if n, ok := GroupChannelCount("71"); !ok || n != 8 {
		t.Errorf("71 = %d %v", n, ok)
	}
	if _, ok := GroupChannelCount("ZZ"); ok {
		t.Error("ZZ should be unknown")
	}
}

func TestAudioSDP(t *testing.T) {
	f := Format{
		Encoding: L24, SampleRate: 48000, Channels: 2, PacketTime: PacketTime1ms,
		ChannelOrder: "SMPTE2110.(ST)",
	}
	if got := f.RTPMap(101).String(); got != "101 L24/48000/2" {
		t.Errorf("rtpmap = %q", got)
	}
	if got := f.PTime(); got != "1" {
		t.Errorf("ptime = %q, want 1", got)
	}

	f125 := f
	f125.PacketTime = PacketTime125us
	if got := f125.PTime(); got != "0.125" {
		t.Errorf("ptime 125us = %q, want 0.125", got)
	}

	m := f.MediaDescription(50000, 101)
	if m.Media != "audio" || m.Port != 50000 {
		t.Errorf("media = %+v", m)
	}
	parsed, err := ParseMediaDescription(m)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Encoding != L24 || parsed.SampleRate != 48000 || parsed.Channels != 2 {
		t.Errorf("parsed = %+v", parsed)
	}
	if parsed.PacketTime != PacketTime1ms {
		t.Errorf("parsed ptime = %v", parsed.PacketTime)
	}
	if parsed.ChannelOrder != "SMPTE2110.(ST)" {
		t.Errorf("parsed channel-order = %q", parsed.ChannelOrder)
	}
}
