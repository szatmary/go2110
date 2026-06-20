# Hardening checklist (independent conformance audit)

Tracks each defect, the change made, and the spec-pinned regression test.
`main` must always pass `go build ./... && go vet ./... && go test ./...`.

## video (ST 2110-20)
- [x] **V1** UDP-size overflow — payload budget = MaxUDP − 8 (UDP) − 12 (RTP). `packetizer.go`: added `udpHeaderLen`, fixed budget + BPM clamp. Tests: `TestV1_UDPDatagramWithinLimit`, `TestV1_ExtendedUDPLimit`.
- [x] **V2** Interlaced footgun — `Packetize()` returns `ErrInterlacedUsePacketizeFields`. Test: `TestV2_InterlacedPacketizeErrors`.
- [x] **V3** colorimetry Required — `FMTP` defaults empty colorimetry to BT709. Test: `TestV3_ColorimetryAlwaysEmitted`.
- [x] **V4** SRD Length==0 rejected unless single SRD header (`payload.go` Marshal + `ErrZeroLength`). Test: `TestV4_ZeroLengthSRD`.
- [x] **V5** XYZ only at depth 12/16/16f (`pgroup.go`). Test: `TestV5_XYZDepthRestriction`.
- [x] **V6** Sample-order on-wire vectors for 4:4:4, 4:2:0, RGB, Key (`frame_v6_test.go`, pinned to RFC 4175 §6.2). Test: `TestV6_SampleOrderVectors`.

## ttml (ST 2110-43 / RFC 8759)
- [x] **T1** SDP `codecs=<profile>` (MUST) — `MediaDescription` now emits `a=fmtp:<pt> charset=utf-8;codecs=<profile>` (default im2t), exact per RFC 8759 Figure 5. Test: `TestT1_SDPCodecs`.
- [x] **T2** Depacketize validates shared timestamp (§4.2), consecutive seq (§8), rejects zero-length docs (§6). Errors `ErrTimestampMismatch`/`ErrSequenceGap`/`ErrEmptyDocument`. Test: `TestT2_DepacketizeValidation`.

## cv22 (ST 2110-22)
- [x] **C1** exactframerate fmtp param round-trips; parse error surfaced (no longer swallowed); `TestRoundTrip` now compares `FrameRate`. Tests: `TestC1_FractionalFrameRateRoundTrip`, `TestC1_MalformedExactFrameRateErrors`, updated `TestRoundTrip`/`TestExampleMediaSection`.

## timing21 (ST 2110-21)
- [ ] **P1** TROFF in microseconds.
- [ ] **P2** Drain grid epoch-anchored.
- [ ] **P3** Wire up validators + integration test (NetworkCompatible/VRXCompliant).
- [ ] **P4** ReadSchedule keeps TROFFSET; interlaced TRODEFAULT table.

## audio (ST 2110-30)
- [ ] **A1** Clause 7 conformance levels (A/B/C/AX/BX/CX); Validate rejects out-of-spec.
- [ ] **A2** Remove bogus PacketTime250us/333us + misleading comment.

## aes3 (ST 2110-31)
- [ ] **E1** ptime emits 0.12 not 0.125.
- [ ] **E2** ptime/level validation.

## anc (ST 2110-40 / RFC 8331)
- [ ] **N1** Marker-bit API + compliant keep-alive (ANC_Count 0, marker set).
- [ ] **N2** Reject F=0b01; verify Checksum on parse.

## media (ST 2110-10)
- [ ] **M1** SampleTimestamp honors Rate.
- [ ] **M2** Wire clock-derived timestamps into the sender.

## sdp / rtp (stretch — S1)
- [ ] **S1** RFC 7273 generality + 2-byte RTP extension vector.
