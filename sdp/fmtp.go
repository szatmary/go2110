package sdp

import "strings"

// FormatParameters models an a=fmtp attribute (RFC 4566 §6): a format (the RTP
// payload type for RTP/AVP) followed by media-type-specific parameters. ST
// 2110-20 §7.1 specifies the parameter syntax used by the 2110 suite: entries
// separated by "; " (semicolon followed by a space), each either "name=value"
// or a standalone "name", with no trailing semicolon.
type FormatParameters struct {
	// Format is the format token, e.g. "112" for an RTP dynamic payload type.
	Format string
	// Params holds the parameters in declaration order.
	Params []Param
}

// Param is a single fmtp parameter. HasValue is false for a standalone flag
// such as "interlace".
type Param struct {
	Name     string
	Value    string
	HasValue bool
}

// String renders the fmtp value (without the leading "fmtp:").
func (f FormatParameters) String() string {
	parts := make([]string, 0, len(f.Params))
	for _, p := range f.Params {
		if p.HasValue {
			parts = append(parts, p.Name+"="+p.Value)
		} else {
			parts = append(parts, p.Name)
		}
	}
	if len(parts) == 0 {
		return f.Format
	}
	return f.Format + " " + strings.Join(parts, "; ")
}

// ParseFormatParameters parses an fmtp attribute value. It tolerates optional
// whitespace around the "; " separators (the standards' PDFs wrap these lines).
func ParseFormatParameters(v string) FormatParameters {
	format, rest, ok := strings.Cut(strings.TrimSpace(v), " ")
	f := FormatParameters{Format: format}
	if !ok {
		return f
	}
	for _, item := range strings.Split(rest, ";") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		name, val, has := strings.Cut(item, "=")
		f.Params = append(f.Params, Param{Name: strings.TrimSpace(name), Value: strings.TrimSpace(val), HasValue: has})
	}
	return f
}

// Get returns the value of the named parameter and whether it is present with a
// value.
func (f FormatParameters) Get(name string) (string, bool) {
	for _, p := range f.Params {
		if p.Name == name && p.HasValue {
			return p.Value, true
		}
	}
	return "", false
}

// Has reports whether a parameter (flag or value) with the given name exists.
func (f FormatParameters) Has(name string) bool {
	for _, p := range f.Params {
		if p.Name == name {
			return true
		}
	}
	return false
}

// Set adds or replaces a value-bearing parameter, preserving position on
// replace and appending otherwise.
func (f *FormatParameters) Set(name, value string) {
	for i := range f.Params {
		if f.Params[i].Name == name {
			f.Params[i] = Param{Name: name, Value: value, HasValue: true}
			return
		}
	}
	f.Params = append(f.Params, Param{Name: name, Value: value, HasValue: true})
}

// SetFlag adds a standalone (value-less) parameter if not already present.
func (f *FormatParameters) SetFlag(name string) {
	if f.Has(name) {
		return
	}
	f.Params = append(f.Params, Param{Name: name})
}

// FormatParameters parses the media's fmtp attribute, if present.
func (m *MediaDescription) FormatParameters() (FormatParameters, bool) {
	if v, ok := m.GetAttribute("fmtp"); ok {
		return ParseFormatParameters(v), true
	}
	return FormatParameters{}, false
}
