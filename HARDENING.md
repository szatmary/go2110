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
- [x] **P1** TROFF emitted in microseconds (§8.2, confirmed from the SMPTE PDF). `SetTROFF(float64 seconds)` + `ParseTROFF`. Test: `TestP1_TROFFMicroseconds`.
- [x] **P2** `MaxCInst` drains on an Epoch-anchored grid (N×TDRAIN), epoch-absolute timebase, O(packets) via floor() (no longer iterates from 0). Test: `TestP2_DrainGridEpochAnchored`.
- [x] **P3** Public `Validate`/`ComplianceReport` entry point wiring VRXFull/CMax/ReadSchedule/MaxCInst/VRX; integration test packetizes a real frame and checks `NetworkCompatible`/`VRXCompliant` for a Narrow sender. Test: `TestP3_PacketizerStreamIsCompliant`.
- [x] **P4** `ReadSchedule` keeps TROFFSET (`Params.TROffset`); interlaced/PsF 1125-line TRODEFAULT (§6.3.3 Table 1) = INT((TotalLines−Height)/2)/TotalLines × TFRAME. Tests: `TestP4_ReadScheduleKeepsTROFFSET`, `TestP4_InterlacedTRODEFAULT`. (525/625 SD rows carry an extra bottom-alignment term not implemented — see DECISIONS.md.)

## audio (ST 2110-30)
- [x] **A1** Clause 7 Table 2 levels (A/AX/B/BX/C/CX) in `levels.go`; `Validate` enforces rate∈{48k,96k}, channels≤64, and a matching level. Tests: `TestA1_ConformanceLevels`, `TestA1_ValidateRejectsOutOfSpec`.
- [x] **A2** Removed bogus `PacketTime250us`/`333us`; comment now cites §7 Table 2 accurately (only 1 ms / 125 µs). Test: `TestA2_BogusPacketTimesRemoved`.

## aes3 (ST 2110-31)
- [x] **E1** `PTime` rounds to 2 dp with midway-down → 48k/125µs emits "0.12" (Table 1 / Table 3 note). Test: `TestE1_PTime0_12`.
- [x] **E2** `Validate` enforces Table 1 (rate, periods) + per-mode Subframe-sequence ceiling; `ConformanceLevels` represents §7 Table 3 (A/AX/B/BX/C/CX/D/DX) in `levels.go`. Tests: `TestE2_ValidatePacketTime`, `TestE2_ConformanceLevels`.

## anc (ST 2110-40 / RFC 8331)
- [ ] **N1** Marker-bit API + compliant keep-alive (ANC_Count 0, marker set).
- [ ] **N2** Reject F=0b01; verify Checksum on parse.

## media (ST 2110-10)
- [ ] **M1** SampleTimestamp honors Rate.
- [ ] **M2** Wire clock-derived timestamps into the sender.

## sdp / rtp (stretch — S1)
- [ ] **S1** RFC 7273 generality + 2-byte RTP extension vector.
