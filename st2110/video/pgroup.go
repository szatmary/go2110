package video

import "errors"

// Sampling is the value of the ST 2110-20 "sampling" media-type parameter
// (§7.4.1): the color (sub-)sampling structure of the stream.
type Sampling string

// Sampling values defined in ST 2110-20 §7.4.1.
const (
	SamplingYCbCr444   Sampling = "YCbCr-4:4:4"
	SamplingYCbCr422   Sampling = "YCbCr-4:2:2"
	SamplingYCbCr420   Sampling = "YCbCr-4:2:0"
	SamplingCLYCbCr444 Sampling = "CLYCbCr-4:4:4"
	SamplingCLYCbCr422 Sampling = "CLYCbCr-4:2:2"
	SamplingCLYCbCr420 Sampling = "CLYCbCr-4:2:0"
	SamplingICtCp444   Sampling = "ICtCp-4:4:4"
	SamplingICtCp422   Sampling = "ICtCp-4:2:2"
	SamplingICtCp420   Sampling = "ICtCp-4:2:0"
	SamplingRGB        Sampling = "RGB"
	SamplingXYZ        Sampling = "XYZ"
	SamplingKey        Sampling = "KEY"
)

// Depth is the value of the ST 2110-20 "depth" media-type parameter (§7.4.2).
type Depth string

// Depth values defined in ST 2110-20 §7.4.2.
const (
	Depth8   Depth = "8"
	Depth10  Depth = "10"
	Depth12  Depth = "12"
	Depth16  Depth = "16"
	Depth16f Depth = "16f"
)

// Bits returns the number of bits per sample for the depth (16f is 16 bits).
func (d Depth) Bits() int {
	switch d {
	case Depth8:
		return 8
	case Depth10:
		return 10
	case Depth12:
		return 12
	case Depth16, Depth16f:
		return 16
	}
	return 0
}

// family classifies a sampling value by its pgroup geometry.
type family int

const (
	fam444 family = iota // 3 components per pixel, no sub-sampling
	fam422               // 4:2:2 horizontal chroma sub-sampling
	fam420               // 4:2:0 horizontal+vertical chroma sub-sampling
	famKey               // single component
)

func samplingFamily(s Sampling) (family, bool) {
	switch s {
	case SamplingYCbCr444, SamplingCLYCbCr444, SamplingICtCp444, SamplingRGB, SamplingXYZ:
		return fam444, true
	case SamplingYCbCr422, SamplingCLYCbCr422, SamplingICtCp422:
		return fam422, true
	case SamplingYCbCr420, SamplingCLYCbCr420, SamplingICtCp420:
		return fam420, true
	case SamplingKey:
		return famKey, true
	}
	return 0, false
}

// Pgroup describes the geometry of a pgroup for a sampling/depth combination
// (ST 2110-20 §6.2): its octet size, the number of full-bandwidth pixels it
// covers, and the number of component samples it contains.
type Pgroup struct {
	// Octets is the pgroup size in octets.
	Octets int
	// Pixels is the number of full-bandwidth pixels covered (horizontal for
	// 4:4:4/4:2:2/Key; horizontal × 2 rows for 4:2:0, counting both rows).
	Pixels int
	// Samples is the number of component samples in the pgroup.
	Samples int
}

// ErrUnsupportedFormat is returned for a sampling/depth combination not defined
// by ST 2110-20.
var ErrUnsupportedFormat = errors.New("st2110/video: unsupported sampling/depth combination")

// PgroupFor returns the pgroup geometry for a sampling/depth combination per the
// construction tables of ST 2110-20 §6.2.3 (4:4:4), §6.2.4 (4:2:2), §6.2.5
// (4:2:0) and §6.2.6 (Key).
func PgroupFor(s Sampling, d Depth) (Pgroup, error) {
	fam, ok := samplingFamily(s)
	if !ok {
		return Pgroup{}, ErrUnsupportedFormat
	}
	bits := d.Bits()
	if bits == 0 {
		return Pgroup{}, ErrUnsupportedFormat
	}
	// XYZ is defined only at 12-bit and 16-bit/16f depth (ST 2110-20 §6.2.3,
	// Table 1); it has no 8-bit or 10-bit form.
	if s == SamplingXYZ && d != Depth12 && d != Depth16 && d != Depth16f {
		return Pgroup{}, ErrUnsupportedFormat
	}
	var octets, pixels int
	switch fam {
	case fam444: // Table 1
		switch d {
		case Depth8:
			octets, pixels = 3, 1
		case Depth10:
			octets, pixels = 15, 4
		case Depth12:
			octets, pixels = 9, 2
		case Depth16, Depth16f:
			octets, pixels = 6, 1
		default:
			return Pgroup{}, ErrUnsupportedFormat
		}
	case fam422: // Table 2
		switch d {
		case Depth8:
			octets, pixels = 4, 2
		case Depth10:
			octets, pixels = 5, 2
		case Depth12:
			octets, pixels = 6, 2
		case Depth16, Depth16f:
			octets, pixels = 8, 2
		default:
			return Pgroup{}, ErrUnsupportedFormat
		}
	case fam420: // Table 3 (no 16-bit form)
		switch d {
		case Depth8:
			octets, pixels = 6, 4
		case Depth10:
			octets, pixels = 15, 8
		case Depth12:
			octets, pixels = 9, 4
		default:
			return Pgroup{}, ErrUnsupportedFormat
		}
	case famKey: // Table 4
		switch d {
		case Depth8:
			octets, pixels = 1, 1
		case Depth10:
			octets, pixels = 5, 4
		case Depth12:
			octets, pixels = 3, 2
		case Depth16, Depth16f:
			octets, pixels = 2, 1
		default:
			return Pgroup{}, ErrUnsupportedFormat
		}
	}
	return Pgroup{Octets: octets, Pixels: pixels, Samples: octets * 8 / bits}, nil
}

// packSamples serializes samples into a contiguous big-endian (MSB-first)
// bitstream of depth-bit values, returning ceil(len*depth/8) octets. This is the
// exact pgroup bit layout shown in the ST 2110-20 figures (e.g. Figure 3 for the
// 4:2:2 10-bit pgroup).
func packSamples(samples []uint16, depth int) []byte {
	out := make([]byte, (len(samples)*depth+7)/8)
	bitpos := 0
	mask := uint32(1)<<uint(depth) - 1
	for _, s := range samples {
		v := uint32(s) & mask
		for i := depth - 1; i >= 0; i-- {
			if v>>uint(i)&1 == 1 {
				out[bitpos>>3] |= 1 << (7 - uint(bitpos&7))
			}
			bitpos++
		}
	}
	return out
}

// unpackSamples is the inverse of packSamples: it reads count depth-bit samples
// MSB-first from data.
func unpackSamples(data []byte, count, depth int) []uint16 {
	out := make([]uint16, count)
	bitpos := 0
	for n := 0; n < count; n++ {
		var v uint16
		for i := 0; i < depth; i++ {
			bit := data[bitpos>>3] >> (7 - uint(bitpos&7)) & 1
			v = v<<1 | uint16(bit)
			bitpos++
		}
		out[n] = v
	}
	return out
}
