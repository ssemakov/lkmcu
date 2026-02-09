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

- The media pipeline wraps LiveKit’s RTP receivers and connects them to the clock mapper, drift controller, and jitter buffer.
- Its responsibility is to convert an unaligned, out-of-order, network-timed packet stream into ordered, time-synchronized samples/frames ready for decoding.

At this point, the stream is still compressed (Opus, VP8/H.264).

---

## 3. Packet Sorting and Jitter Buffer

RTP packets rarely arrive in perfect order or timing. The jitter buffer handles this:

1. Ingress: Each packet is pushed into the buffer (`Push(packet)`).
2. Ordering: Packets are sorted by their NTP-derived timestamp, not arrival time.
3. Latency Window: Packets are held for ~80–120 ms to allow for reordering and delay variance.
4. Playout: When `Pop(target_time)` is called (aligned to MCU master clock), the next frame’s worth of packets are released to the decoder.

This ensures smooth, continuous media even if packets arrive late or unevenly spaced.

---

## 4. RTP → NTP Clock Mapping

The MCU needs to know when each RTP timestamp occurred in real time. That mapping is built using RTCP Sender Reports (SR) from each publisher:

```
NTP(t_rtp) = offset + (t_rtp - base_rtp) / clock_rate
```

Where:

- `base_rtp`, `base_ntp`: the latest SR anchor, normalized for 32‑bit RTP rollover.
- `clock_rate`: codec clock (e.g., audio 48 kHz, video 90 kHz).
- `offset = base_ntp - base_rtp / clock_rate`.

On every new SR the mapper updates `base_rtp`/`base_ntp`, recomputes `offset`, and continues to convert intervening RTP timestamps via linear extrapolation within a bounded window. See RTP→NTP Clock Mapping & Drift Control for the full model and edge cases (rollover, monotonic guards, SR loss): `docs/rtp_clock_mapping.md`.

Each SSRC maintains:

- Last SR,
- Clock rate,
- Wrap-count for 32-bit rollover,
- Monotonic guard to prevent backward jumps.

This mapping is updated on new SRs and extrapolated between them (up to 10 s).

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

---

## 6. Decoding and Mixing

Once the jitter buffer outputs time-aligned packets:

1. Audio Path:
   - Decode Opus → PCM (48 kHz).
   - Mix multiple active speakers with limiter & gain control.
   - Encode mixed output → single Opus track for publication.

2. Video Path:
   - Decode VP8/H.264 → raw YUV frames.
   - Align frames on NTP timestamps.
   - Composite frames according to the stage layout (GPU compositor).
   - Encode output (720p@30) → publish via LiveKit SDK.

All frames/samples reference the same master clock, so audio and video stay locked.

---

## 7. Output & Publication

The mixed/composited outputs are published back to the LiveKit room (as the MCU’s own tracks). Typical latency breakdown:

- Input jitter/transport: 80–120 ms
- Decode + mix/compose + encode: 40–120 ms
- Publish to subscribers: 80–120 ms
  → Total end-to-end ≈ 300–400 ms, within real-time interaction range.

---

## 8. Observability & Control

Key metrics and signals surfaced:

- `media_drift_ppm{ssrc}` (gauge)
- `media_drift_over_6ppm_seconds_total{ssrc}` (counter)
- `media_drift_over_11ppm_seconds_total{ssrc}`
- `media_drift_corrections_total{action,ssrc}`
- `media_drift_resets_total{reason,ssrc}`
- `media_clock_sr_age_seconds`
- `jitter_buffer_occupancy_ms`
- `late_drops_total`, `duplicates_total`
- Drift correction count/duration

Operators can adjust:

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
 [Drift Controller] ↔ Master Clock
   ↓
 [Jitter Buffer]
   ↓
 [Decoder]
   ↓
 [Mixer / Compositor]
   ↓
 [Encoder]
   ↓
 LiveKit SDK (Publish mixed A/V)
```

---

In essence: The MCU Agent continuously subscribes to dynamic room tracks, stabilizes their timing via RTP→NTP mapping + jitter buffering + drift control, decodes and merges them, and republishes perfectly synchronized, low-latency composite streams back to the room.
