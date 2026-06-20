# Engineering Decisions

Non-obvious decisions made while implementing go2110. Newest on top.

## Conformance hardening (independent audit)

- **ST 2110-21 §6.3.3 SD interlaced TRODEFAULT (P4).** The 525/625-line interlaced
  rows of Table 1 add an extra bottom-of-active alignment term to the base
  `INT((total−HEIGHT)/2)/total × TFRAME` offset (Note 2: SD visual lines are
  aligned to the bottom of the active area, with variable top blanking). That
  addend could not be reliably extracted from the SMPTE PDF's math layout (it
  renders as a dangling "+" with no recoverable operand, even via per-glyph bbox
  extraction). Only the 1125-line BT.709-6 interlaced/PsF case — the interlaced
  raster actually carried over ST 2110 — is implemented and tested; SD rasters are
  out of scope rather than guessed. `TROffsetDefaultSeconds` returns the verified
  1125 value for interlaced streams.
- **ST 2110-21 TROFF unit (P1).** Confirmed from the §8.2 text in the public SMPTE
  PDF: TROFF is "a positive integer number of microseconds", not RTP-clock ticks.
- **colorimetry default (V3).** Empty `Format.Colorimetry` defaults to `BT709` so
  the Required §7.2 parameter is always present; chosen over returning an error to
  keep `FMTP` total.
- **TTML codecs profile (T1).** Default processor profile is `im2t` (IMSC 1.1
  Text), matching the RFC 8759 §11.2.1 example SDP; the fmtp value is emitted with
  a bare `;` separator (no space) to match the normative example exactly.

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
    `st2110/anc` (ST 2110-40), `st2110/timing21` (ST 2110-21),
    `st2110/aes3` (ST 2110-31), `st2110/cv22` (ST 2110-22),
    `st2110/fastmeta` (ST 2110-41), `st2110/ttml` (ST 2110-43).

## Framework standards (-22, -41, -43)

- ST 2110-22 (compressed video), -41 (fast metadata), and -43 (timed text) are
  transport/SDP frameworks: -22 wraps an external compressed codec (JPEG XS,
  VC-2), -43 defers the wire format to RFC 8759, and -41 carries opaque Data
  Item Packages. The packages therefore implement the framework's own payload
  header / SDP exactly, and treat the inner codec/data-item *contents* as opaque
  bytes — faithful to what the standards actually define.

## 2110-21 numerics

- VRXFULL / CMAX use exact integer arithmetic (the standard's INT() == floor),
  computed from the frame-rate rational, so results land on the correct side of
  the floor boundary without floating-point error. The leaky-bucket (CINST) and
  Virtual Receiver Buffer simulators use float64 time, which is adequate for
  compliance checking of emission-time sequences.

## ANC / AES3 / video bit packing

- ANC 10-bit ST 291-1 words, AES3 AM824 subframes, and video pgroup samples are
  all serialized MSB-first into a contiguous bitstream; each package has a small
  bit reader/writer (or reuses the pattern) so the wire layout matches the spec
  figures exactly.

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
