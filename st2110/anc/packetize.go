package anc

import "github.com/szatmary/go2110/rtp"

// PacketizeOptions controls ANC RTP packetization (RFC 8331 / ST 2110-40 §5).
type PacketizeOptions struct {
	// PayloadType is the RTP dynamic payload type.
	PayloadType uint8
	// SSRC is the RTP synchronization source.
	SSRC uint32
	// Timestamp is the RTP timestamp, contemporaneous with the related video
	// frame/field (ST 2110-40 §5.4).
	Timestamp uint32
	// SequenceNumber is the low 16 bits of the RTP sequence number (the high bits
	// are carried in PayloadHeader.ExtendedSequenceNumber).
	SequenceNumber uint16
	// Marker sets the RTP marker bit, which "indicates the last ANC data RTP
	// packet for a frame (progressive) or field (interlaced)" (RFC 8331 §2,
	// ST 2110-40 §5.5). Set it on the final ANC RTP packet of each frame/field.
	Marker bool
}

// Packetize builds one ANC data RTP packet from the payload header and ANC data
// packets, setting the marker bit per opts.Marker. To carry more than 255 ANC
// packets in a frame/field, emit several RTP packets with the same timestamp and
// consecutive sequence numbers, setting Marker only on the last (RFC 8331 §2.1).
func Packetize(h PayloadHeader, packets []Packet, opts PacketizeOptions) (rtp.Packet, error) {
	payload, err := Marshal(h, packets)
	if err != nil {
		return rtp.Packet{}, err
	}
	return rtp.Packet{
		Header: rtp.Header{
			Version:        rtp.Version,
			PayloadType:    opts.PayloadType,
			SequenceNumber: opts.SequenceNumber,
			Timestamp:      opts.Timestamp,
			SSRC:           opts.SSRC,
			Marker:         opts.Marker,
		},
		Payload: payload,
	}, nil
}

// KeepAlive returns a compliant ANC keep-alive RTP packet: ANC_Count 0 (and
// Length 0) with the marker bit set, i.e. a last-of-frame/field RTP packet that
// carries no actual ANC data (RFC 8331 §2.1, ST 2110-40 §5.5). It is used to
// signal frame/field boundaries when a frame/field has no ANC data. The F field
// and extended sequence number come from h.
func KeepAlive(h PayloadHeader, opts PacketizeOptions) (rtp.Packet, error) {
	opts.Marker = true // a keep-alive is always the last packet of the field/frame
	return Packetize(h, nil, opts)
}
