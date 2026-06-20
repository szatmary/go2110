package video

import (
	"github.com/szatmary/go2110/rtp"
	"github.com/szatmary/go2110/st2110/media"
)

// PacketizeFrame packetizes a progressive frame, deriving the shared RTP
// timestamp through the media clock from the zero-based frame index rather than
// requiring the caller to supply opts.Timestamp. Every packet of the frame
// carries clock.FrameTimestamp(frameIndex, Format.ExactFrameRate), the
// spec-correct sampling-instant timestamp of ST 2110-10 §7.6.1. opts.Timestamp
// is ignored (overwritten). For interlaced/PsF formats use PacketizeFrameFields.
func (pf *PackedFrame) PacketizeFrame(clock media.Clock, frameIndex int64, opts PacketizeOptions) ([]rtp.Packet, error) {
	opts.Timestamp = clock.FrameTimestamp(frameIndex, pf.Format.ExactFrameRate)
	return pf.Packetize(opts)
}

// PacketizeFrameFields packetizes an interlaced/PsF frame, deriving each field's
// RTP timestamp through the media clock from the zero-based frame index: the
// first field uses FieldTimestamp(frameIndex, 0, …) and the second
// FieldTimestamp(frameIndex, 1, …), per ST 2110-10 §7.6.1. opts.Timestamp is
// ignored.
func (pf *PackedFrame) PacketizeFrameFields(clock media.Clock, frameIndex int64, opts PacketizeOptions) ([]rtp.Packet, error) {
	fps := pf.Format.ExactFrameRate
	first := clock.FieldTimestamp(frameIndex, 0, fps)
	second := clock.FieldTimestamp(frameIndex, 1, fps)
	return pf.PacketizeFields(opts, first, second)
}
