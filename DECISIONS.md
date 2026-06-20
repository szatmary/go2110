# Engineering Decisions

Non-obvious decisions made while implementing go2110. Newest on top.

## Dependencies

- **Zero external dependencies.** Everything is built on the Go standard library.
  Any future dependency must be justified here.

## Package layout

- `rtp/` — RFC 3550 RTP packet (header + payload), independent of any 2110 media.
- `sdp/` — RFC 4566 Session Description Protocol with the 2110/RFC 7273 attributes.
- `st2110/` — the SMPTE-specific layer:
  - `st2110/media` — timing model shared by all essence types (RTP clock, media
    clock, frame-rate tick math).
  - `st2110/video` (ST 2110-20), `st2110/audio` (ST 2110-30),
    `st2110/anc` (ST 2110-40), `st2110/timing21` (ST 2110-21).

Rationale: `rtp` and `sdp` are reusable RFC building blocks with no SMPTE
coupling; the `st2110/*` packages layer the standard's constraints on top. This
keeps spec boundaries explicit and matches the document structure (2110-10
defines the common system; -20/-30/-40 are essence-specific).

## RTP design

- `rtp.Header.Marshal`/`Unmarshal` operate on `[]byte` and return byte counts,
  mirroring `encoding`-style APIs, avoiding allocations on the hot path where
  practical. A higher-level `Packet` bundles header + payload.
- CSRC list and RFC 8285 header extensions are modeled explicitly; the one-byte
  and two-byte extension forms are both supported because 2110-10 §6 references
  RFC 8285 header extensions.

## 2110-20 video

- pgroup packing is implemented as a bit-writer/bit-reader over the component
  sample slice. Components are stored as `uint16` in a flat per-line buffer in
  sampling order; the packer serializes them MSB-first into the contiguous
  pgroup bitstream exactly as the spec figures show. This is depth-agnostic and
  bit-exact for 8/10/12/16-bit.
- The frame model stores samples as a planar/flat `[]uint16` keyed by component;
  conversion helpers handle 4:2:2/4:4:4/4:2:0 sub-sampling geometry.
