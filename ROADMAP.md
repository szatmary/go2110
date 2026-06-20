# go2110 Roadmap

Implementation of the SMPTE ST 2110 suite in idiomatic Go. Each item ships with
table-driven tests asserting exact, spec-defined byte layouts, using worked
examples from the standards where available.

Legend: `[x]` done · `[~]` in progress · `[ ]` not started

## Foundation

- [x] 1. Project scaffold — `go.mod` (go 1.26), package layout, planning docs, CI.
- [x] 2. **RTP (RFC 3550)** — `rtp` package: header marshal/unmarshal (version,
      padding, extension, CSRC, marker, payload type, sequence, timestamp, SSRC,
      RFC 8285 header extensions, padding), table-driven tests.
- [x] 3. **ST 2110-10** — system timing & SDP.
  - [x] `media`/timing: 90 kHz RTP clock, media-clock → RTP timestamp model,
        frame-rate tick increments (incl. 60000/1001 alternating 1501/1502).
  - [x] `sdp` package (RFC 4566): generate + parse SDP, incl. 2110 attributes
        `ts-refclk` (ptp / traceable / localmac), `mediaclk` (direct/sender),
        `source-filter`, `group:DUP`, `mid`. Tests against the §Annex B example.
- [x] 4. **ST 2110-20** — uncompressed active video.
  - [x] RTP payload header: extended sequence number + 1..3 SRD headers
        (Length / F / RowNumber / C / Offset) per §6.1.4. Marshal/unmarshal.
  - [x] pgroup pack/unpack: YCbCr-4:2:2 (8/10/12/16), 4:4:4, RGB, 4:2:0, Key.
  - [x] Packetizer (frame → RTP packets) and depacketizer (packets → frame),
        GPM and BPM packing modes. Bit-exact tests.
  - [x] SDP fmtp media-type parameters for video (`raw`): sampling, depth,
        width, height, exactframerate, colorimetry, PM, SSN, TCS, RANGE, etc.

## Beyond the foundation

- [x] 5. **ST 2110-30** — PCM audio (AES67 profile): packetization, packet time,
      channel counts, SDP (`L16`/`L24`, `ptime`, `channel-order`).
- [x] 6. **ST 2110-40** — ancillary data (ST 291-1): ANC RTP payload, ANC packet
      headers (DID/SDID/DC/UDW + checksum), SDP (`smpte291`, `DID_SDID`).
- [x] 7. **ST 2110-21** — sender timing models (N / NL / W): traffic-shaping
      parameters, TROFF / CMAX / etc., compliance helpers.
- [ ] 8. **ST 2110-31 / -22 / -41 / -43** — AES3 transparent, constant-bitrate
      compressed video, fast metadata, timed text. (spec dependent)

See `DECISIONS.md` for non-obvious engineering decisions and `PROGRESS.md` for the
running log and the next concrete step.
