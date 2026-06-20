package video

import "github.com/szatmary/go2110/st2110/media"

// PackingMode is the value of the ST 2110-20 "PM" media-type parameter (§6.3).
type PackingMode string

// Packing modes defined in ST 2110-20 §6.3.
const (
	PackingGPM PackingMode = "2110GPM" // General Packing Mode (§6.3.2)
	PackingBPM PackingMode = "2110BPM" // Block Packing Mode (§6.3.3)
)

// BPMBlock is the Block Packing Mode block size in octets (ST 2110-20 §6.3.3:
// the sum of SRD lengths shall be a multiple of 180 octets).
const BPMBlock = 180

// BPMBlocksPerPacket is the number of 180-octet blocks per BPM packet (§6.3.3:
// "a payload of 7 x 180 octets per packet shall be used").
const BPMBlocksPerPacket = 7

// StandardUDPLimit and ExtendedUDPLimit are the UDP datagram size limits of
// ST 2110-10 §6.3/§6.4 (Annex A): 1460 and 8960 octets respectively.
const (
	StandardUDPLimit = 1460
	ExtendedUDPLimit = 8960
)

// Format describes an ST 2110-20 video stream. The geometry fields (Sampling,
// Depth, Width, Height, Interlaced, Segmented) drive packing; the remaining
// fields are descriptive SDP media-type parameters (§7).
type Format struct {
	Sampling Sampling
	Depth    Depth
	Width    int
	Height   int

	// Interlaced is the §7.3 "interlace" flag (interlaced or PsF when set).
	Interlaced bool
	// Segmented is the §7.3 "segmented" flag (PsF; requires Interlaced).
	Segmented bool

	// ExactFrameRate is the §7.2 exactframerate.
	ExactFrameRate media.Rational
	// Colorimetry is the §7.5 colorimetry value (e.g. "BT709").
	Colorimetry string
	// TCS is the §7.6 transfer characteristic system (default "SDR").
	TCS string
	// Range is the §7.3 RANGE value (default "NARROW").
	Range string
	// PackingMode is the §6.3 PM value (default PackingGPM).
	PackingMode PackingMode
	// SSN is the §7.2 SMPTE Standard Number (default "ST2110-20:2017").
	SSN string
	// MaxUDP is the §8.6 MAXUDP value in octets (0 ⇒ Standard UDP Size Limit).
	MaxUDP int
	// PAR is the §7.3 pixel aspect ratio (e.g. "12:11"; empty ⇒ "1:1").
	PAR string
}

// Pgroup returns the pgroup geometry for the format.
func (f Format) Pgroup() (Pgroup, error) { return PgroupFor(f.Sampling, f.Depth) }

// planeCount returns the number of component planes for the sampling.
func (f Format) planeCount() int {
	if fam, _ := samplingFamily(f.Sampling); fam == famKey {
		return 1
	}
	return 3
}

// planeDims returns the (width, height) of each component plane for the format.
// For 4:2:2 the chroma planes (index 0 and 2) are half width; for 4:2:0 they are
// half width and half height (ST 2110-20 §6.2).
func (f Format) planeDims() [][2]int {
	fam, _ := samplingFamily(f.Sampling)
	switch fam {
	case fam444:
		return [][2]int{{f.Width, f.Height}, {f.Width, f.Height}, {f.Width, f.Height}}
	case fam422:
		cw := f.Width / 2
		return [][2]int{{cw, f.Height}, {f.Width, f.Height}, {cw, f.Height}}
	case fam420:
		cw, ch := f.Width/2, f.Height/2
		return [][2]int{{cw, ch}, {f.Width, f.Height}, {cw, ch}}
	default: // key
		return [][2]int{{f.Width, f.Height}}
	}
}

// hPixelsPerPgroup returns the number of horizontal full-bandwidth pixels
// advanced by one pgroup, used to compute the SRD Offset field. For 4:2:0 a
// pgroup spans two sample rows, so its horizontal advance is Pixels/2.
func (f Format) hPixelsPerPgroup(pg Pgroup) int {
	if fam, _ := samplingFamily(f.Sampling); fam == fam420 {
		return pg.Pixels / 2
	}
	return pg.Pixels
}
