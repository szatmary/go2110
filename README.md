# go2110

An idiomatic, well-tested Go implementation of the **SMPTE ST 2110** suite —
"Professional Media Over Managed IP Networks" — built entirely on the Go standard
library (zero external dependencies).

> Written 100% by Claude. Spec conformance is verified with table-driven,
> byte-exact tests tied to worked examples and figures in the standards.

```
go get github.com/szatmary/go2110
```

## Packages

| Package | Standard / RFC | What it does |
|---|---|---|
| `rtp` | RFC 3550, RFC 8285 | RTP fixed header, CSRC, header extensions, packet (with padding) marshal/unmarshal |
| `sdp` | RFC 4566, RFC 7273 | SDP marshal/parse + typed 2110 attributes: `rtpmap`, `fmtp`, `ts-refclk`, `mediaclk`, `source-filter`, `group:DUP`, `mid` |
| `st2110/media` | ST 2110-10 §7 | 90 kHz media/RTP clock, frame/field RTP-timestamp math (exact, incl. 60000/1001 alternating 1501/1502), `exactframerate` rationals |
| `st2110/video` | ST 2110-20 | Uncompressed video: RTP payload header (ext-seq + SRD), pgroup pack/unpack (4:2:2, 4:4:4, RGB, 4:2:0, Key; 8/10/12/16-bit), packetizer/depacketizer (GPM + BPM), SDP fmtp |
| `st2110/audio` | ST 2110-30 (AES67) | PCM audio L16/L24, packet-time/channel handling, packetize/depacketize, `channel-order` (SMPTE2110) |
| `st2110/anc` | ST 2110-40, RFC 8331 | Ancillary data: ANC RTP payload, ST 291-1 10-bit words with parity + checksum, SDP (`smpte291`, `DID_SDID`) |
| `st2110/timing21` | ST 2110-21 | Sender types N / NL / W: VRXFULL & CMAX (exact), gapped/linear Packet Read Schedules, Network-Compatibility & Virtual-Receiver-Buffer simulators, TP/TROFF |
| `st2110/aes3` | ST 2110-31 | AES3 transparent transport (AM824 32-bit subframes), packetize/depacketize, SDP (`AM824`) |
| `st2110/cv22` | ST 2110-22 | Constant-bit-rate compressed video transport framework: SDP (`b=AS`, TP, width/height), bit-rate helpers |
| `st2110/fastmeta` | ST 2110-41 | Fast metadata framework: Data Item Packages (Type/K/Length), NCM helpers, SDP (`ST2110-41`, `DIT`) |
| `st2110/ttml` | ST 2110-43, RFC 8759 | Timed text (TTML2): RFC 8759 payload, UTF-8-safe fragmentation, keep-alive, SDP (`ttml+xml`) |

## Example: packetize a frame of uncompressed video

```go
f := video.Format{
    Sampling:       video.SamplingYCbCr422,
    Depth:          video.Depth10,
    Width:          1280, Height: 720,
    ExactFrameRate: media.Rational{Num: 60000, Den: 1001},
    Colorimetry:    "BT709", TCS: "SDR", PackingMode: video.PackingGPM,
}
frame := video.NewFrame(f)        // fill frame.Planes with samples…
packed, _ := frame.Pack()
clock := media.VideoClock()
pkts, _ := packed.Packetize(video.PacketizeOptions{
    PayloadType:   96,
    Timestamp:     clock.FrameTimestamp(0, f.ExactFrameRate),
    StartSequence: 0,
})
// pkts is []rtp.Packet — pkt.Marshal() gives wire bytes.
```

## Project docs

- [`ROADMAP.md`](ROADMAP.md) — the plan and what's done.
- [`PROGRESS.md`](PROGRESS.md) — running log and known gaps.
- [`DECISIONS.md`](DECISIONS.md) — non-obvious engineering decisions.

## Development

```
go build ./... && go vet ./... && go test ./...
```

CI (GitHub Actions) runs the same on every push. `main` is always green.

## Status & scope

The full roadmap (ST 2110-10/-20/-21/-22/-30/-31/-40/-41/-43) is implemented.
Known limitations: video packetization currently covers **progressive** scan
(interlaced/PsF is a documented gap); the `cv22`, `fastmeta`, and `ttml`
packages implement the 2110 transport/SDP framework, while inner codec payloads
(e.g. JPEG XS, VC-2) are out of scope of those framework standards.
