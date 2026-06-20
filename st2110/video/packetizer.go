package video

import (
	"errors"
	"sort"

	"github.com/szatmary/go2110/rtp"
)

// rtpFixedHeaderLen is the size of the RTP header used by the packetizer (no
// CSRC list, no header extension).
const rtpFixedHeaderLen = 12

// Errors for packetization.
var (
	ErrBudgetTooSmall = errors.New("st2110/video: UDP size limit too small for one pgroup")
	ErrBPMAlignment   = errors.New("st2110/video: Block Packing Mode requires the 1260-octet payload to be a multiple of the pgroup size")
	ErrBPMTooManyRows = errors.New("st2110/video: Block Packing Mode packet would need more than three SRD headers")
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

// Packetize splits a packed frame into RTP packets per ST 2110-20 §6.1 and the
// packing mode in Format.PackingMode (GPM by default; BPM uses 7×180-octet
// payloads). The marker bit is set on the final packet of the frame (§6.1.2).
func (pf *PackedFrame) Packetize(opts PacketizeOptions) ([]rtp.Packet, error) {
	payloadBudget := pf.Format.maxRTPPacket(opts) - rtpFixedHeaderLen
	og := pf.PgroupOctets
	if payloadBudget < extSeqLen+srdHeaderLen+og {
		return nil, ErrBudgetTooSmall
	}

	bpm := pf.Format.PackingMode == PackingBPM
	bpmTarget := BPMBlock * BPMBlocksPerPacket // 1260
	if bpm {
		if bpmTarget%og != 0 {
			return nil, ErrBPMAlignment
		}
		// BPM forbids the Extended UDP size limit (§6.3.3).
		if payloadBudget+rtpFixedHeaderLen > StandardUDPLimit {
			payloadBudget = StandardUDPLimit - rtpFixedHeaderLen
		}
	}

	var packets []rtp.Packet
	ext := opts.StartSequence
	lineIdx, byteOff := 0, 0
	totalLines := len(pf.Lines)

	for lineIdx < totalLines {
		var ph PayloadHeader
		low, high := SplitSequenceNumber(ext)
		ph.ExtendedSequenceNumber = high

		var payloadData []byte
		rem := payloadBudget - extSeqLen
		dataSoFar := 0
		for len(ph.SRDs) < MaxSRD {
			if lineIdx >= totalLines {
				break
			}
			ln := pf.Lines[lineIdx]
			avail := len(ln.Data) - byteOff

			maxData := rem - srdHeaderLen
			if bpm {
				if r := bpmTarget - dataSoFar; r < maxData {
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
			if bpm && dataSoFar >= bpmTarget {
				break
			}
		}
		if bpm && dataSoFar < bpmTarget && lineIdx < totalLines && len(ph.SRDs) == MaxSRD {
			return nil, ErrBPMTooManyRows
		}

		hdrBytes, err := ph.Marshal()
		if err != nil {
			return nil, err
		}
		payload := make([]byte, 0, len(hdrBytes)+len(payloadData))
		payload = append(payload, hdrBytes...)
		payload = append(payload, payloadData...)

		pkt := rtp.Packet{
			Header: rtp.Header{
				Version:        rtp.Version,
				PayloadType:    opts.PayloadType,
				SequenceNumber: low,
				Timestamp:      opts.Timestamp,
				SSRC:           opts.SSRC,
				Marker:         lineIdx >= totalLines, // last packet of frame
			},
			Payload: payload,
		}
		packets = append(packets, pkt)
		ext++
	}
	return packets, nil
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
