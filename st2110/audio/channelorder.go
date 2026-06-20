package audio

import (
	"errors"
	"strconv"
	"strings"
)

// ChannelOrderConvention is the <convention> token of the RFC 3190 channel-order
// parameter. ST 2110-30 §6.2.2 defines the SMPTE2110 convention.
const ChannelOrderConvention = "SMPTE2110"

// ErrBadChannelOrder is returned for a malformed channel-order value.
var ErrBadChannelOrder = errors.New("st2110/audio: malformed channel-order")

// groupChannelCounts maps the fixed Channel Grouping Symbols of ST 2110-30
// Table 1 to their channel counts. Undefined groups (U01..U64) are handled
// separately.
var groupChannelCounts = map[string]int{
	"M":    1,  // Mono
	"DM":   2,  // Dual Mono
	"ST":   2,  // Standard Stereo
	"LtRt": 2,  // Matrix Stereo
	"51":   6,  // 5.1 Surround
	"71":   8,  // 7.1 Surround
	"222":  24, // 22.2 Surround
	"SGRP": 4,  // One SDI audio group
}

// GroupChannelCount returns the number of audio channels represented by a
// Channel Grouping Symbol from ST 2110-30 Table 1, including the Undefined
// "Unn" form. It returns ok=false for an unknown symbol.
func GroupChannelCount(symbol string) (int, bool) {
	if n, ok := groupChannelCounts[symbol]; ok {
		return n, true
	}
	if len(symbol) == 3 && symbol[0] == 'U' {
		if n, err := strconv.Atoi(symbol[1:]); err == nil && n >= 1 && n <= 64 {
			return n, true
		}
	}
	return 0, false
}

// ChannelOrder is a parsed channel-order value: a convention and an ordered list
// of Channel Grouping Symbols (ST 2110-30 §6.2.2).
type ChannelOrder struct {
	Convention string
	Groups     []string
}

// String renders the channel-order value, e.g. "SMPTE2110.(51,ST)".
func (c ChannelOrder) String() string {
	conv := c.Convention
	if conv == "" {
		conv = ChannelOrderConvention
	}
	return conv + ".(" + strings.Join(c.Groups, ",") + ")"
}

// TotalChannels returns the sum of channel counts of all groups, or an error if
// any group symbol is unknown.
func (c ChannelOrder) TotalChannels() (int, error) {
	total := 0
	for _, g := range c.Groups {
		n, ok := GroupChannelCount(g)
		if !ok {
			return 0, ErrBadChannelOrder
		}
		total += n
	}
	return total, nil
}

// ParseChannelOrder parses a channel-order value such as
// "SMPTE2110.(M,M,M,M,ST,U02)".
func ParseChannelOrder(v string) (ChannelOrder, error) {
	v = strings.TrimSpace(v)
	conv, rest, ok := strings.Cut(v, ".")
	if !ok {
		return ChannelOrder{}, ErrBadChannelOrder
	}
	if !strings.HasPrefix(rest, "(") || !strings.HasSuffix(rest, ")") {
		return ChannelOrder{}, ErrBadChannelOrder
	}
	inner := rest[1 : len(rest)-1]
	co := ChannelOrder{Convention: conv}
	if inner == "" {
		return co, nil
	}
	for _, g := range strings.Split(inner, ",") {
		co.Groups = append(co.Groups, strings.TrimSpace(g))
	}
	return co, nil
}
