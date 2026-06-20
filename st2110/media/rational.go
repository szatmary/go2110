package media

import (
	"errors"
	"strconv"
	"strings"
)

// ErrBadRational is returned when a frame-rate string cannot be parsed.
var ErrBadRational = errors.New("media: invalid rational value")

// ParseExactFrameRate parses an ST 2110-20 exactframerate value (§7.2). An
// integer frame rate is a single decimal number (e.g. "25"); a non-integer rate
// is a ratio of two integers separated by "/" (e.g. "30000/1001"). The returned
// Rational has Den == 1 for the integer form.
func ParseExactFrameRate(s string) (Rational, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Rational{}, ErrBadRational
	}
	numStr, denStr, hasSlash := strings.Cut(s, "/")
	num, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil || num <= 0 {
		return Rational{}, ErrBadRational
	}
	if !hasSlash {
		return Rational{Num: num, Den: 1}, nil
	}
	den, err := strconv.ParseInt(denStr, 10, 64)
	if err != nil || den <= 0 {
		return Rational{}, ErrBadRational
	}
	return Rational{Num: num, Den: den}, nil
}

// String formats a Rational as an ST 2110-20 exactframerate value: a single
// integer when Den == 1, otherwise "Num/Den".
func (r Rational) String() string {
	if r.Den == 1 {
		return strconv.FormatInt(r.Num, 10)
	}
	return strconv.FormatInt(r.Num, 10) + "/" + strconv.FormatInt(r.Den, 10)
}

// Float64 returns the rational as a floating-point value (Num/Den).
func (r Rational) Float64() float64 {
	if r.Den == 0 {
		return 0
	}
	return float64(r.Num) / float64(r.Den)
}
