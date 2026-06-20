package anc

// bitWriter serializes values into a big-endian (MSB-first) bitstream, growing
// its buffer as needed. It is used for the 32-bit ANC location header and the
// contiguous 10-bit ST 291-1 data words.
type bitWriter struct {
	buf   []byte
	nbits int
}

// write appends the low n bits of v, most-significant bit first.
func (w *bitWriter) write(v uint32, n int) {
	for i := n - 1; i >= 0; i-- {
		if w.nbits>>3 >= len(w.buf) {
			w.buf = append(w.buf, 0)
		}
		if v>>uint(i)&1 == 1 {
			w.buf[w.nbits>>3] |= 1 << (7 - uint(w.nbits&7))
		}
		w.nbits++
	}
}

// alignTo pads with zero bits until the bit position is a multiple of boundary.
func (w *bitWriter) alignTo(boundary int) {
	for w.nbits%boundary != 0 {
		if w.nbits>>3 >= len(w.buf) {
			w.buf = append(w.buf, 0)
		}
		w.nbits++
	}
}

// bytes returns the serialized octets (the final partial octet is already
// zero-padded by write).
func (w *bitWriter) bytes() []byte { return w.buf }

// bitReader reads a big-endian (MSB-first) bitstream.
type bitReader struct {
	buf []byte
	pos int // bit position
}

// read returns the next n bits as an unsigned integer, or ok=false if fewer than
// n bits remain.
func (r *bitReader) read(n int) (uint32, bool) {
	if r.pos+n > len(r.buf)*8 {
		return 0, false
	}
	var v uint32
	for i := 0; i < n; i++ {
		bit := r.buf[r.pos>>3] >> (7 - uint(r.pos&7)) & 1
		v = v<<1 | uint32(bit)
		r.pos++
	}
	return v, true
}

// alignTo advances the read position to the next multiple of boundary bits,
// skipping word_align padding.
func (r *bitReader) alignTo(boundary int) {
	for r.pos%boundary != 0 && r.pos < len(r.buf)*8 {
		r.pos++
	}
}
