package video

import (
	"errors"
	"sort"

	"github.com/szatmary/go2110/rtp"
)

// rtpFixedHeaderLen is the size of the RTP header used by the packetizer (no
// CSRC list, no header extension).
const rtpFixedHeaderLen = 12

// udpHeaderLen is the size of the UDP header (RFC 768). ST 2110-10 §6.3 measures
// the Standard/Extended UDP Size Limit against the complete UDP datagram, i.e.
// the 8-octet UDP header + the RTP packet (header + payload). The per-packet
// payload budget must therefore reserve both the UDP and RTP headers.
const udpHeaderLen = 8

// Errors for packetization.
var (
	ErrBudgetTooSmall = errors.New("st2110/video: UDP size limit too small for one pgroup")
	ErrBPMAlignment   = errors.New("st2110/video: Block Packing Mode requires the 1260-octet payload to be a multiple of the pgroup size")
	ErrBPMTooManyRows = errors.New("st2110/video: Block Packing Mode packet would need more than three SRD headers")
	// ErrInterlacedUsePacketizeFields is returned by Packetize for interlaced or
	// PsF formats: a single shared RTP timestamp is correct only for progressive
	// frames (ST 2110-10 §7.6.1), so the two fields must be emitted with their own
	// timestamps via PacketizeFields.
	ErrInterlacedUsePacketizeFields = errors.New("st2110/video: interlaced/PsF format requires PacketizeFields (each field needs its own RTP timestamp)")
)

// PacketizeOptions controls RTP packetization of a frame.
type PacketizeOptions struct {
	// PayloadType is the RTP dynamic payload type (ST 2110-10 §6.2).
	PayloadType uint8
	// SSRC is the RTP synchronization source.
	SSRC uint32
	// Timestamp is the RTP timestamp shared by every packet of the frame
	// (ST 2110-20 §6.1.3); use media.Clock to compute it.
	Timestamp uint32
	// StartSequence is the 32-bit extended sequence number of the first packet.
	StartSequence uint32
	// MaxUDP overrides Format.MaxUDP / the Standard UDP Size Limit when > 0.
	MaxUDP int
}

// maxRTPPacket returns the maximum RTP packet size in octets.
func (f Format) maxRTPPacket(opts PacketizeOptions) int {
	switch {
	case opts.MaxUDP > 0:
		return opts.MaxUDP
	case f.MaxUDP > 0:
		return f.MaxUDP
	default:
		return StandardUDPLimit
	}
}

// shaper holds the per-frame packing constraints shared across fields.
type shaper struct {
	payloadBudget int
	og            int
	bpm           bool
	bpmTarget     int
}

// newShaper validates the packing constraints for the frame.
func (pf *PackedFrame) newShaper(opts PacketizeOptions) (shaper, error) {
	s := shaper{
		payloadBudget: pf.Format.maxRTPPacket(opts) - udpHeaderLen - rtpFixedHeaderLen,
		og:            pf.PgroupOctets,
		bpm:           pf.Format.PackingMode == PackingBPM,
		bpmTarget:     BPMBlock * BPMBlocksPerPacket, // 1260
	}
	if s.payloadBudget < extSeqLen+srdHeaderLen+s.og {
		return shaper{}, ErrBudgetTooSmall
	}
	if s.bpm {
		if s.bpmTarget%s.og != 0 {
			return shaper{}, ErrBPMAlignment
		}
		// BPM forbids the Extended UDP size limit (§6.3.3). The datagram size
		// includes the UDP and RTP headers (§6.3).
		if s.payloadBudget+rtpFixedHeaderLen+udpHeaderLen > StandardUDPLimit {
			s.payloadBudget = StandardUDPLimit - udpHeaderLen - rtpFixedHeaderLen
		}
	}
	return s, nil
}

// Packetize splits a packed frame into RTP packets per ST 2110-20 §6.1 and the
// packing mode in Format.PackingMode (GPM by default; BPM uses 7×180-octet
// payloads). The marker bit is set on the final packet of the frame (§6.1.2).
//
// For interlaced/PsF streams (Format.Interlaced), use PacketizeFields instead so
// that each field receives its own RTP timestamp; Packetize emits all lines under
// a single timestamp, which is correct only for progressive frames.
func (pf *PackedFrame) Packetize(opts PacketizeOptions) ([]rtp.Packet, error) {
	if pf.Format.Interlaced {
		return nil, ErrInterlacedUsePacketizeFields
	}
	s, err := pf.newShaper(opts)
	if err != nil {
		return nil, err
	}
	packets, _, err := pf.packetizeLines(pf.Lines, opts, s, opts.Timestamp, opts.StartSequence)
	return packets, err
}

// PacketizeFields packetizes an interlaced or PsF frame as two fields/segments,
// each carrying its own RTP timestamp (ST 2110-10 §7.6.1): the temporally first
// field uses firstTimestamp and the second uses secondTimestamp (equal for PsF).
// The marker bit is set on the last packet of each field, and sequence numbers
// run continuously across both fields.
func (pf *PackedFrame) PacketizeFields(opts PacketizeOptions, firstTimestamp, secondTimestamp uint32) ([]rtp.Packet, error) {
	s, err := pf.newShaper(opts)
	if err != nil {
		return nil, err
	}
	var first, second []Line
	for _, ln := range pf.Lines {
		if ln.Field {
			second = append(second, ln)
		} else {
			first = append(first, ln)
		}
	}
	p0, ext, err := pf.packetizeLines(first, opts, s, firstTimestamp, opts.StartSequence)
	if err != nil {
		return nil, err
	}
	p1, _, err := pf.packetizeLines(second, opts, s, secondTimestamp, ext)
	if err != nil {
		return nil, err
	}
	return append(p0, p1...), nil
}

// packetizeLines packetizes a single field (or whole progressive frame) worth of
// lines under one timestamp, setting the marker bit on the last packet. It
// returns the packets and the next extended sequence number.
func (pf *PackedFrame) packetizeLines(lines []Line, opts PacketizeOptions, s shaper, timestamp uint32, startExt uint32) ([]rtp.Packet, uint32, error) {
	og := s.og
	var packets []rtp.Packet
	ext := startExt
	lineIdx, byteOff := 0, 0
	totalLines := len(lines)

	for lineIdx < totalLines {
		var ph PayloadHeader
		low, high := SplitSequenceNumber(ext)
		ph.ExtendedSequenceNumber = high

		var payloadData []byte
		rem := s.payloadBudget - extSeqLen
		dataSoFar := 0
		for len(ph.SRDs) < MaxSRD {
			if lineIdx >= totalLines {
				break
			}
			ln := lines[lineIdx]
			avail := len(ln.Data) - byteOff

			maxData := rem - srdHeaderLen
			if s.bpm {
				if r := s.bpmTarget - dataSoFar; r < maxData {
					maxData = r
				}
			}
			maxData -= maxData % og
			if maxData <= 0 {
				break
			}
			take := avail
			if take > maxData {
				take = maxData
			}

			offsetPix := (byteOff / og) * pf.HPixelsPerGroup
			ph.SRDs = append(ph.SRDs, SRDHeader{
				Length:    uint16(take),
				Field:     ln.Field,
				RowNumber: ln.RowNumber,
				Offset:    uint16(offsetPix),
			})
			payloadData = append(payloadData, ln.Data[byteOff:byteOff+take]...)
			byteOff += take
			rem -= srdHeaderLen + take
			dataSoFar += take

			if byteOff >= len(ln.Data) {
				lineIdx++
				byteOff = 0
			} else {
				// Line not exhausted: the packet is full for this line. Only
				// reason to keep going would be more budget, which there isn't.
				break
			}
			if s.bpm && dataSoFar >= s.bpmTarget {
				break
			}
		}
		if s.bpm && dataSoFar < s.bpmTarget && lineIdx < totalLines && len(ph.SRDs) == MaxSRD {
			return nil, ext, ErrBPMTooManyRows
		}

		hdrBytes, err := ph.Marshal()
		if err != nil {
			return nil, ext, err
		}
		payload := make([]byte, 0, len(hdrBytes)+len(payloadData))
		payload = append(payload, hdrBytes...)
		payload = append(payload, payloadData...)

		pkt := rtp.Packet{
			Header: rtp.Header{
				Version:        rtp.Version,
				PayloadType:    opts.PayloadType,
				SequenceNumber: low,
				Timestamp:      timestamp,
				SSRC:           opts.SSRC,
				Marker:         lineIdx >= totalLines, // last packet of field/frame
			},
			Payload: payload,
		}
		packets = append(packets, pkt)
		ext++
	}
	return packets, ext, nil
}

// lineKey identifies a sample row (or 4:2:0 row-pair) within a frame.
type lineKey struct {
	field bool
	row   uint16
}

// Depacketize reassembles a PackedFrame from the RTP packets of a single frame
// (or field). Packets may arrive with Sample Row Data Segments in any order; the
// SRD Offset and Length fields are used to place each segment. The format
// supplies the pgroup geometry needed to map pixel offsets to byte offsets.
func Depacketize(packets []rtp.Packet, f Format) (*PackedFrame, error) {
	pg, err := f.Pgroup()
	if err != nil {
		return nil, err
	}
	hpix := f.hPixelsPerPgroup(pg)
	og := pg.Octets

	type buf struct {
		data []byte
	}
	lines := map[lineKey]*buf{}

	for i := range packets {
		var ph PayloadHeader
		n, err := ph.Unmarshal(packets[i].Payload)
		if err != nil {
			return nil, err
		}
		data := packets[i].Payload[n:]
		off := 0
		for _, srd := range ph.SRDs {
			seg := data[off : off+int(srd.Length)]
			off += int(srd.Length)
			key := lineKey{field: srd.Field, row: srd.RowNumber}
			b := lines[key]
			if b == nil {
				b = &buf{}
				lines[key] = b
			}
			byteOff := (int(srd.Offset) / hpix) * og
			end := byteOff + len(seg)
			if len(b.data) < end {
				grown := make([]byte, end)
				copy(grown, b.data)
				b.data = grown
			}
			copy(b.data[byteOff:end], seg)
		}
	}

	pf := &PackedFrame{Format: f, PgroupOctets: og, HPixelsPerGroup: hpix}
	keys := make([]lineKey, 0, len(lines))
	for k := range lines {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].field != keys[j].field {
			return !keys[i].field // field 0 before field 1
		}
		return keys[i].row < keys[j].row
	})
	for _, k := range keys {
		pf.Lines = append(pf.Lines, Line{RowNumber: k.row, Field: k.field, Data: lines[k].data})
	}
	return pf, nil
}
