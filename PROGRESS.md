# Progress Log

A running log so a fresh VM can continue. Newest entry on top.

## Current state

- Bootstrapping the project: scaffold + planning docs being committed first.

## Next concrete step

- Implement the `rtp` package (RFC 3550 header marshal/unmarshal) with
  table-driven tests, then commit + push.

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
