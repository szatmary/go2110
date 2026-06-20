package media

import "testing"

func TestFrameTimestampIntegerRate(t *testing.T) {
	c := VideoClock()
	fps := Rational{25, 1}
	// 90000 / 25 = 3600 ticks per frame, exactly.
	for n := int64(0); n < 5; n++ {
		got := c.FrameTimestamp(n, fps)
		want := uint32(n * 3600)
		if got != want {
			t.Errorf("FrameTimestamp(%d) = %d, want %d", n, got, want)
		}
	}
}

func TestFrameDurationAlternates5994(t *testing.T) {
	// ST 2110-10 §7.6.1 NOTE 1: 60/1.001 Hz frame periods alternate between
	// increments of 1501 and 1502 ticks of a 90 kHz clock.
	c := VideoClock()
	fps := Rational{60000, 1001}
	want := []uint32{1501, 1502, 1501, 1502, 1501, 1502}
	for n := int64(0); n < int64(len(want)); n++ {
		if got := c.FrameDurationTicks(n, fps); got != want[n] {
			t.Errorf("FrameDurationTicks(%d) = %d, want %d", n, got, want[n])
		}
	}
	// Cumulative timestamps: 0, 1501, 3003, 4504, 6006, ...
	wantTS := []uint32{0, 1501, 3003, 4504, 6006, 7507}
	for n := int64(0); n < int64(len(wantTS)); n++ {
		if got := c.FrameTimestamp(n, fps); got != wantTS[n] {
			t.Errorf("FrameTimestamp(%d) = %d, want %d", n, got, wantTS[n])
		}
	}
}

func TestFrameTimestamp30000_1001(t *testing.T) {
	// 29.97: 90000*1001/30000 = 3003 ticks exactly per frame.
	c := VideoClock()
	fps := Rational{30000, 1001}
	if got := c.FrameDurationTicks(0, fps); got != 3003 {
		t.Errorf("29.97 duration = %d, want 3003", got)
	}
}

func TestFieldTimestampInterlaced(t *testing.T) {
	c := VideoClock()
	fps := Rational{30000, 1001} // 59.94 fields
	// Half frame period: floor(90000*1001/(2*30000)) = floor(1501.5) = 1501.
	if off := c.FieldHalfOffset(fps); off != 1501 {
		t.Fatalf("FieldHalfOffset = %d, want 1501", off)
	}
	first := c.FieldTimestamp(0, 0, fps)
	second := c.FieldTimestamp(0, 1, fps)
	if first != 0 {
		t.Errorf("first field ts = %d, want 0", first)
	}
	if second != 1501 {
		t.Errorf("second field ts = %d, want 1501", second)
	}
}

func TestParseExactFrameRate(t *testing.T) {
	tests := []struct {
		in      string
		want    Rational
		wantErr bool
	}{
		{"25", Rational{25, 1}, false},
		{"30000/1001", Rational{30000, 1001}, false},
		{"60000/1001", Rational{60000, 1001}, false},
		{" 50 ", Rational{50, 1}, false},
		{"", Rational{}, true},
		{"0", Rational{}, true},
		{"30/0", Rational{}, true},
		{"abc", Rational{}, true},
	}
	for _, tc := range tests {
		got, err := ParseExactFrameRate(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseExactFrameRate(%q) expected error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseExactFrameRate(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseExactFrameRate(%q) = %+v, want %+v", tc.in, got, tc.want)
		}
		// Re-parsing the canonical String() form must reproduce the value.
		if rt, err := ParseExactFrameRate(got.String()); err != nil || rt != tc.want {
			t.Errorf("round-trip %q -> %q -> %+v (err %v)", tc.in, got.String(), rt, err)
		}
	}
}

func TestRationalString(t *testing.T) {
	if s := (Rational{25, 1}).String(); s != "25" {
		t.Errorf("got %q, want 25", s)
	}
	if s := (Rational{60000, 1001}).String(); s != "60000/1001" {
		t.Errorf("got %q, want 60000/1001", s)
	}
}
