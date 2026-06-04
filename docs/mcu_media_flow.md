# MCU Media Flow Overview

## 1. Role of the MCU Agent

The MCU Agent acts as a special LiveKit participant:

- It monitors the stage layout and room snapshot via control-plane events.
- Based on who’s visible or audible, it subscribes/unsubscribes to the corresponding audio/video tracks from the room.
- Once subscribed, the LiveKit SDK delivers the raw RTP streams (per track, per SSRC) to the MCU’s media layer.

This forms the input side of the MCU media pipeline.

---

## 2. Hand-off to Media Pipeline

Each incoming RTP stream is handed off to the `media` package:

- The current track pipeline wraps LiveKit’s RTP track and RTCP receiver and connects them to the clock mapper, jitter buffer, depacketizer, and decoder interfaces.
- Its current responsibility is to convert an unaligned, out-of-order, network-timed RTP packet stream into ordered packets ready for depacketization and decode.

At this point, the stream is still compressed (Opus, VP8/H.264).

---

## 3. Packet Sorting and Jitter Buffer

RTP packets rarely arrive in perfect order or timing. The jitter buffer handles this:

1. Ingress: Each packet is pushed into the buffer (`Push(packet)`).
2. Ordering: Packets are sorted by their NTP-derived timestamp, not arrival time.
3. Latency Window: Packets are held for the configured target depth (80 ms by default, widened to 200 ms while mapper timing is degraded).
4. Playout: The current audio loop ticks every 10 ms and calls `Pop(now)` for each active audio SSRC, draining all packets that are ready at that playout cursor.
5. Handoff: Released packets flow into the selected depacketizer, then into the decoder interface. The current Opus depacketizer and decoder are stub implementations; PCM production and mixing are future pipeline stages.

This establishes the input-side timing and decode integration points for smooth media once concrete codec implementations are added.

---

## 4. RTP → NTP Clock Mapping

The MCU needs to know when each RTP timestamp occurred in real time. That mapping is built using RTCP Sender Reports (SR) from each publisher:

```
NTP(t_rtp) = base_ntp + (extended_rtp(t_rtp) - base_rtp) / clock_rate
```

Where:

- `base_rtp`, `base_ntp`: the latest SR anchor, normalized for 32‑bit RTP rollover.
- `clock_rate`: codec clock (e.g., audio 48 kHz, video 90 kHz).

On every accepted SR the mapper updates `base_rtp`/`base_ntp` and continues to convert intervening RTP timestamps via linear extrapolation until SR timeout marks the mapping degraded. See RTP→NTP Clock Mapping & Drift Control for the full model and edge cases (rollover, monotonic guards, SR loss): `docs/rtp_clock_mapping.md`.

Each SSRC maintains:

- Last SR,
- Clock rate,
- Wrap-count for 32-bit rollover,
- Monotonic guard to prevent backward jumps.

This mapping is updated on new SRs and uses a 5 s SR timeout by default.

---

## 5. Drift Controller

Even with perfect mapping, different publishers’ clocks drift over time. The drift controller compares expected NTP vs actual arrival time (master clock):

- Keeps a sliding window (≈ 5 min) of deltas.
- Estimates drift rate (ppm) using EWMA or linear regression.
- If drift exceeds ±6 ppm, triggers correction.

Integration:

- Audio: call once per decoded frame or every N packets (e.g., every 20–40 ms).
- Video: call at frame boundaries (on PTS ready for playout).
- Use the packet’s mapped time for expected (e.g., `expected = NTPFromMapper`) and receive time from the MCU master clock for arrival (e.g., `arrival = time.Now()`).
- Tip: do not call for every RTP packet—sample at a stable cadence to reduce noise and overhead.

Correction mechanisms:

- Audio: Slightly adjust resampling ratio or insert/drop 10 ms frames with crossfade. Compute resample ratio as `ratio = 1 - ppm*1e-6` and clamp to ±0.5%. Feed this to your resampler (e.g., speexdsp, soxr, or a Go resampler). If there’s residual bursty error, schedule one 10 ms insert/drop per ≥2 min with a short crossfade.
- Video: Nudge frame presentation timestamps toward the MCU’s master clock by up to 2 ms per frame (VideoNudgePerFrame). Only drop/dup frames if residual drift > 20 ms and no keyframe is in sight; otherwise request a keyframe and let the mapper’s reset/monotonic guard carry you.

The goal: keep all streams within ±40 ms/hour of MCU time.

The drift controller exists in `internal/media/clock/drift.go`, but this branch does not yet wire it into the track pipeline or mixer loop.

---

## 6. Decoding and Mixing

Once the jitter buffer outputs time-aligned packets:

1. Audio Path:
   - The shared audio render loop takes one `now` value from the MCU master clock for the current tick.
   - For each active audio track, it drains all ready packets from the jitter buffer via `Pop(now)`.
   - Those packets flow into a per-track depacketizer, then into the decoder interface.
   - The current Opus depacketizer and decoder implementations are stubs that compile and are selected by MIME type.
   - Producing fixed 48 kHz PCM blocks, mixing active tracks, and encoding mixed output are future work.

Current audio flow after playout scheduling:

`jitter buffer Pop(now) -> depacketizer -> decoder Accept`

2. Video Path:
   - Video subscription decisions still exist, but this branch does not create video processing pipelines.
   - Decode, frame alignment, composition, and output encoding are future work.

The shared timing model is intended to keep future audio and video processing locked to the same master clock.

---

## 7. Output & Publication

The mixed/composited outputs will be published back to the LiveKit room (as the MCU’s own tracks). Target latency budget:

- Input jitter/transport: 80–120 ms
- Decode + mix/compose + encode: 40–120 ms
- Publish to subscribers: 80–120 ms
  → Total end-to-end ≈ 300–400 ms, within real-time interaction range.

---

## 8. Observability & Control

Key metrics and signals planned for this area:

- `media_drift_ppm{ssrc}` (gauge)
- `media_drift_over_6ppm_seconds_total{ssrc}` (counter)
- `media_drift_over_11ppm_seconds_total{ssrc}`
- `media_drift_corrections_total{action,ssrc}`
- `media_drift_resets_total{reason,ssrc}`
- `media_clock_sr_age_seconds`
- `jitter_buffer_occupancy_ms`
- `late_drops_total`, `duplicates_total`
- Drift correction count/duration

Planned operator controls:

- Buffer depths,
- SR timeout,
- Drift budget,
- Role-based latency targets.

---

## 9. Summary Dataflow Diagram (textual)

```
LiveKit SDK (RTP)
   ↓
 [Media Pipeline]
   ↓
 [Clock Mapper]  ← RTCP SRs
   ↓
 [Jitter Buffer]
   ↓
 [Depacketizer]
   ↓
 [Decoder Interface]
   ↓
 [PCM / Mix / Encode] (future)
   ↓
 LiveKit SDK (Publish mixed A/V)
```

---

In essence: the current branch lets the MCU Agent subscribe to dynamic room tracks, establish RTP/RTCP input pipelines, stabilize packet timing through RTP→NTP mapping and jitter buffering, and hand ready packets into codec-specific depacketizer/decoder interfaces. Concrete PCM output, mixing/composition, drift correction, and republishing remain future pipeline stages.
