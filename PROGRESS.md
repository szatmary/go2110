# Progress Log

A running log so a fresh VM can continue. Newest entry on top.

## Current state

- Roadmap items 1–7 complete: `rtp`, `sdp`, `st2110/media`, `st2110/video`
  (2110-20), `st2110/audio` (2110-30), `st2110/anc` (2110-40), `st2110/timing21`
  (2110-21). All build, vet clean, pass tests on `main`.
- Video: **progressive** packetize/depacketize for 4:2:2 (8/10/12/16), 4:4:4,
  RGB, 4:2:0 (8/10/12), Key, GPM+BPM, bit-exact. Interlaced/PsF is the known gap.
- ANC follows RFC 8331; byte-exact test vector for the payload + 10-bit words.
- timing21: exact-integer VRXFULL/CMAX for N/NL/W (verified on a 1080p50 vector),
  gapped/linear PRS, leaky-bucket (CINST) + VRX compliance simulators.

## Next concrete step

- Roadmap item 8: ST 2110-31 (AES3 transparent), -22 (CBR compressed video),
  -41 (fast metadata), -43 (timed text). These specs are not yet cached under
  spec/ — download from pub.smpte.org first (see the download recipe in the
  run log of Run 1). -31 is the most self-contained; start there.
- Also: add interlaced/PsF support to `st2110/video` (Frame.Pack/Unpack +
  per-field timestamps), replacing the skipped test.

## Run log

### Run 1 (2026-06-20)

- Cloned fresh repo (only had `Initial commit`, README, `.gitignore`).
- Downloaded + cached spec PDFs/text under `spec/` (gitignored): 2110-10, -20,
  -21, -30, -40. Extracted with `pdftotext -layout`.
- Read 2110-10 (timing model §7, SDP §8, UDP limits) and 2110-20 (payload header
  §6.1.4, pgroup tables §6.2, packing modes §6.3, SDP §7) in detail.
- Created `go.mod` (go 1.26), ROADMAP/PROGRESS/DECISIONS.
- Building foundation packages: `rtp`, then `sdp` + timing, then `st2110/video`.

#### Key spec facts captured (so a later run need not re-read)

- **RTP clock for 2110-20 video = 90 kHz.** RTP timestamp same for all packets of
  a frame (or field). 60000/1001 fps → ticks alternate 1501/1502 (90000/ (60000/1001)).
- **2110-20 RTP payload header** (§6.1.4): 16-bit Extended Sequence Number (high
  16 bits of a 32-bit ext seq; RTP header carries low 16). Then 1–3 SRD headers,
  each = SRD Length(16) | F(1) RowNumber(15) | C(1) Offset(15). Length is octets,
  multiple of pgroup size; len 0 only valid as the single header (= no data).
  C=1 means another SRD header follows. Max 3 SRD headers per packet.
- **pgroup sizes** (octets / pixels): 4:2:2 → 8b:4/2, 10b:5/2, 12b:6/2, 16b:8/2.
  4:4:4 → 8b:3/1, 10b:15/4, 12b:9/2, 16b:6/1. 4:2:0 → 8b:6/4, 10b:15/8, 12b:9/4.
  Key → 8b:1/1, 10b:5/4, 12b:3/2, 16b:2/1. Samples packed big-endian, MSB first,
  contiguous bitstream within pgroup.
- **Packing modes**: GPM (2110GPM) general; BPM (2110BPM) SRD total = multiple of
  180 octets, 7×180=1260 payload, no Extended UDP.
- **UDP size limits**: Standard 1460 octets, Extended 8960 octets.
- **SDP 2110-10**: `a=ts-refclk:ptp=IEEE1588-2008:<clockid EUI-64>:<domain>` or
  `...:traceable`, or `a=ts-refclk:localmac=<EUI-48>`. `a=mediaclk:direct=0` or
  `a=mediaclk:sender`. `a=group:DUP a b`, `a=mid:x`, `a=source-filter: incl IN IP4 g s`.
  fmtp 2110-20 example: `sampling=YCbCr-4:2:2; width=1280; height=720;
  exactframerate=60000/1001; depth=10; TCS=SDR; colorimetry=BT709; PM=2110GPM;
  SSN=ST2110-20:2017`.
