package video

import "errors"

// Errors for frame packing.
var (
	ErrBadDimensions = errors.New("st2110/video: frame dimensions incompatible with sampling")
	ErrPlaneSize     = errors.New("st2110/video: component plane has wrong length")
	ErrInterlaced    = errors.New("st2110/video: interlaced/PsF packing not yet implemented")
)

// Frame holds the active video samples of one progressive frame (or a single
// field) as component planes. The component order matches the pgroup Sample
// Order of ST 2110-20 §6.2:
//
//   - YCbCr / CLYCbCr / ICtCp: Planes[0]=Cb, Planes[1]=Y, Planes[2]=Cr.
//   - RGB: Planes[0]=R, Planes[1]=G, Planes[2]=B.
//   - XYZ: Planes[0]=X, Planes[1]=Y, Planes[2]=Z.
//   - Key: Planes[0]=K.
//
// Each plane is row-major with the dimensions reported by Format.planeDims.
// Samples are stored right-justified in uint16 (the low Depth bits).
type Frame struct {
	Format Format
	Planes [][]uint16
}

// NewFrame allocates a zeroed Frame with correctly sized planes for the format.
func NewFrame(f Format) *Frame {
	dims := f.planeDims()
	planes := make([][]uint16, len(dims))
	for i, d := range dims {
		planes[i] = make([]uint16, d[0]*d[1])
	}
	return &Frame{Format: f, Planes: planes}
}

// validate checks that the planes match the format geometry.
func (fr *Frame) validate() error {
	f := fr.Format
	if f.Width <= 0 || f.Height <= 0 {
		return ErrBadDimensions
	}
	fam, ok := samplingFamily(f.Sampling)
	if !ok {
		return ErrUnsupportedFormat
	}
	switch fam {
	case fam422:
		if f.Width%2 != 0 {
			return ErrBadDimensions
		}
	case fam420:
		if f.Width%2 != 0 || f.Height%2 != 0 {
			return ErrBadDimensions
		}
	}
	dims := f.planeDims()
	if len(fr.Planes) != len(dims) {
		return ErrPlaneSize
	}
	for i, d := range dims {
		if len(fr.Planes[i]) != d[0]*d[1] {
			return ErrPlaneSize
		}
	}
	return nil
}

// Line is one Sample Row Data unit: the fully packed pgroup octets for a sample
// row (or, for 4:2:0, a vertically adjacent pair of sample rows), tagged with
// its SRD row number and field.
type Line struct {
	RowNumber uint16
	Field     bool
	Data      []byte
}

// PackedFrame is a frame reduced to packed sample-row data, ready for the
// packetizer. It carries the pgroup geometry needed to derive SRD Offset and
// Length fields.
type PackedFrame struct {
	Format          Format
	PgroupOctets    int
	HPixelsPerGroup int
	Lines           []Line
}

// Pack converts the frame's component planes into packed sample-row data. Only
// progressive frames are supported; interlaced/PsF frames return ErrInterlaced.
func (fr *Frame) Pack() (*PackedFrame, error) {
	if err := fr.validate(); err != nil {
		return nil, err
	}
	if fr.Format.Interlaced {
		return nil, ErrInterlaced
	}
	pg, err := fr.Format.Pgroup()
	if err != nil {
		return nil, err
	}
	bits := fr.Format.Depth.Bits()
	fam, _ := samplingFamily(fr.Format.Sampling)

	pf := &PackedFrame{
		Format:          fr.Format,
		PgroupOctets:    pg.Octets,
		HPixelsPerGroup: fr.Format.hPixelsPerPgroup(pg),
	}
	switch fam {
	case fam420:
		for pair := 0; pair < fr.Format.Height/2; pair++ {
			samples := fr.samples420(pair, pg)
			pf.Lines = append(pf.Lines, Line{RowNumber: uint16(2 * pair), Data: packSamples(samples, bits)})
		}
	default:
		for row := 0; row < fr.Format.Height; row++ {
			samples := fr.samplesRow(fam, row, pg)
			pf.Lines = append(pf.Lines, Line{RowNumber: uint16(row), Data: packSamples(samples, bits)})
		}
	}
	return pf, nil
}

// samplesRow returns the component samples of one sample row in pgroup Sample
// Order, zero-filling a partial final pgroup (ST 2110-20 §6.2.1).
func (fr *Frame) samplesRow(fam family, row int, pg Pgroup) []uint16 {
	f := fr.Format
	w := f.Width
	switch fam {
	case fam444:
		p0, p1, p2 := fr.Planes[0], fr.Planes[1], fr.Planes[2]
		nGroups := (w + pg.Pixels - 1) / pg.Pixels
		out := make([]uint16, 0, nGroups*pg.Samples)
		for g := 0; g < nGroups; g++ {
			for k := 0; k < pg.Pixels; k++ {
				x := g*pg.Pixels + k
				if x < w {
					i := row*w + x
					out = append(out, p0[i], p1[i], p2[i])
				} else {
					out = append(out, 0, 0, 0)
				}
			}
		}
		return out
	case fam422:
		cb, y, cr := fr.Planes[0], fr.Planes[1], fr.Planes[2]
		cw := w / 2
		out := make([]uint16, 0, cw*4)
		for g := 0; g < cw; g++ { // each pgroup covers 2 px = 1 chroma column
			out = append(out, cb[row*cw+g], y[row*w+2*g], cr[row*cw+g], y[row*w+2*g+1])
		}
		return out
	case famKey:
		k := fr.Planes[0]
		nGroups := (w + pg.Pixels - 1) / pg.Pixels
		out := make([]uint16, 0, nGroups*pg.Samples)
		for g := 0; g < nGroups; g++ {
			for j := 0; j < pg.Pixels; j++ {
				x := g*pg.Pixels + j
				if x < w {
					out = append(out, k[row*w+x])
				} else {
					out = append(out, 0)
				}
			}
		}
		return out
	}
	return nil
}

// samples420 returns the component samples for a 4:2:0 row-pair in pgroup Sample
// Order (ST 2110-20 §6.2.5 / Figure 5).
func (fr *Frame) samples420(pair int, pg Pgroup) []uint16 {
	f := fr.Format
	w := f.Width
	cw := w / 2
	cb, y, cr := fr.Planes[0], fr.Planes[1], fr.Planes[2]
	blocksPerPgroup := pg.Pixels / 4 // each 2x2 block is 4 pixels, 1 chroma sample
	row0, row1 := 2*pair, 2*pair+1
	nGroups := cw / blocksPerPgroup
	out := make([]uint16, 0, nGroups*pg.Samples)
	for g := 0; g < nGroups; g++ {
		for b := 0; b < blocksPerPgroup; b++ {
			c := g*blocksPerPgroup + b // chroma column
			x0, x1 := 2*c, 2*c+1
			out = append(out,
				y[row0*w+x0], y[row0*w+x1],
				y[row1*w+x0], y[row1*w+x1],
				cb[pair*cw+c], cr[pair*cw+c],
			)
		}
	}
	return out
}

// Unpack reconstructs a Frame's component planes from packed sample-row data.
func (pf *PackedFrame) Unpack() (*Frame, error) {
	f := pf.Format
	if f.Interlaced {
		return nil, ErrInterlaced
	}
	pg, err := f.Pgroup()
	if err != nil {
		return nil, err
	}
	bits := f.Depth.Bits()
	fam, _ := samplingFamily(f.Sampling)
	fr := NewFrame(f)
	w := f.Width

	for _, ln := range pf.Lines {
		switch fam {
		case fam420:
			pair := int(ln.RowNumber) / 2
			cw := w / 2
			blocksPerPgroup := pg.Pixels / 4
			nGroups := cw / blocksPerPgroup
			samples := unpackSamples(ln.Data, nGroups*pg.Samples, bits)
			cb, y, cr := fr.Planes[0], fr.Planes[1], fr.Planes[2]
			si := 0
			row0, row1 := 2*pair, 2*pair+1
			for g := 0; g < nGroups; g++ {
				for b := 0; b < blocksPerPgroup; b++ {
					c := g*blocksPerPgroup + b
					x0, x1 := 2*c, 2*c+1
					y[row0*w+x0] = samples[si]
					y[row0*w+x1] = samples[si+1]
					y[row1*w+x0] = samples[si+2]
					y[row1*w+x1] = samples[si+3]
					cb[pair*cw+c] = samples[si+4]
					cr[pair*cw+c] = samples[si+5]
					si += 6
				}
			}
		case fam422:
			row := int(ln.RowNumber)
			cw := w / 2
			samples := unpackSamples(ln.Data, cw*4, bits)
			cb, y, cr := fr.Planes[0], fr.Planes[1], fr.Planes[2]
			for g := 0; g < cw; g++ {
				cb[row*cw+g] = samples[4*g]
				y[row*w+2*g] = samples[4*g+1]
				cr[row*cw+g] = samples[4*g+2]
				y[row*w+2*g+1] = samples[4*g+3]
			}
		case fam444:
			row := int(ln.RowNumber)
			nGroups := (w + pg.Pixels - 1) / pg.Pixels
			samples := unpackSamples(ln.Data, nGroups*pg.Samples, bits)
			p0, p1, p2 := fr.Planes[0], fr.Planes[1], fr.Planes[2]
			si := 0
			for g := 0; g < nGroups; g++ {
				for k := 0; k < pg.Pixels; k++ {
					x := g*pg.Pixels + k
					if x < w {
						i := row*w + x
						p0[i], p1[i], p2[i] = samples[si], samples[si+1], samples[si+2]
					}
					si += 3
				}
			}
		case famKey:
			row := int(ln.RowNumber)
			nGroups := (w + pg.Pixels - 1) / pg.Pixels
			samples := unpackSamples(ln.Data, nGroups*pg.Samples, bits)
			k := fr.Planes[0]
			si := 0
			for g := 0; g < nGroups; g++ {
				for j := 0; j < pg.Pixels; j++ {
					x := g*pg.Pixels + j
					if x < w {
						k[row*w+x] = samples[si]
					}
					si++
				}
			}
		}
	}
	return fr, nil
}
