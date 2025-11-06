# Jitter Buffer Design

## Purpose
Describe the buffering strategy that smooths RTP packet arrival variance before decoding/mixing while balancing end-to-end latency. This design depends on the master timeline produced by the [RTP→NTP Clock Mapping & Drift Control](./rtp_clock_mapping.md) document.

## Goals
- Provide stable playout for audio/video under typical network jitter using baseline latency of 80–120 ms.
- Adapt buffer depth based on measured jitter while keeping latency bounded.
- Handle late and duplicate packets deterministically with clear observability.
- Offer configuration knobs per media type.

## Architecture Overview
1. **Ingress**: Jitter buffer receives RTP packets tagged with master-clock presentation timestamps from the mapper.
2. **Queueing**: Packets stored in per-SSRC heaps indexed by timestamp (secondary key: sequence number).
3. **Playback Controller**: Maintains playout cursor on master clock, releases packets when `packet_pts ≤ cursor`.
4. **Adaptation Loop**: Observes inter-arrival variance and adjusts target depth.
5. **Integration**: Decoders/mixers `Pop()` frames aligned to cursor; drift corrections from mapper influence cursor speed but not buffer internals.

## Configuration Defaults
| Parameter | Audio Default | Video Default | Notes |
|-----------|---------------|---------------|-------|
| `initial_target_ms` | 80 | 100 | Buffer fill depth before first playout |
| `min_target_ms` | 40 | 60 | Minimum steady-state latency |
| `max_target_ms` | 160 | 200 | Cap during congestion |
| `growth_step_ms` | 10 | 20 | Increment when jitter exceeds threshold |
| `shrink_step_ms` | 5 | 10 | Decay toward `min_target_ms` in stable periods |
| `late_drop_ms` | 20 | 33 | Drop if packet lag beyond playout cursor + threshold |
| `max_reorder_depth` | 50 packets | 100 packets | Guard against unbounded memory |
| `rebuffer_burst_ms` | 15 | 25 | Pause window when underflow detected |

Expose these values through `internal/config` (e.g., `media.jitter.audio.initial_target_ms`).

## Runtime Behavior
### Startup
- Accumulate packets until occupancy ≥ `initial_target_ms`.
- Report `media_jitter_startup_duration_ms` metric and publish event for observability.

### Adaptive Depth Control
- Track RMS jitter over sliding window (e.g., 500 ms, 50 packets).
- If RMS > `jitter_high_threshold` (default 15 ms audio, 20 ms video), increase `target_ms` by `growth_step_ms` up to `max_target_ms`.
- If RMS < `jitter_low_threshold` (half of high threshold) for `stable_period` (e.g., 3 s), decay `target_ms` by `shrink_step_ms` toward `min_target_ms`.
- Publish metric `media_jitter_target_ms` and logs on significant changes (> 20 ms delta).

### Packet Admission
- `Push(packet)` path:
  1. Validate timestamp monotonicity relative to mapper; reject if conversion degraded beyond `mapper_extrapolation_limit`.
  2. Drop duplicates (same `{timestamp, sequence}`) → increment `media_jitter_duplicates_total`.
  3. If `packet_pts < cursor - late_drop_ms`, drop and increment `media_jitter_late_drops_total`.
  4. Insert into min-heap keyed by `(packet_pts, sequence)`.

### Playout
- `Pop(now)` advances cursor = `now + playout_offset`, where `playout_offset` tracks current `target_ms`.
- Release next packet when heap head `packet_pts ≤ cursor`.
- If heap empty and cursor advanced beyond last packet by `underflow_guard_ms` (e.g., 5 ms), trigger underflow handling:
  - Pause cursor advancement for `rebuffer_burst_ms`.
  - Optionally request keyframe for video.
  - Increment `media_jitter_underflows_total` and emit warning log.

### Degraded Mode
- When mapper reports SR outage or extrapolation beyond window, buffer widens `target_ms` to `max_target_ms` immediately.
- If degraded > configurable timeout (e.g., 30 s), escalate to control plane; consider tearing down track.

## Interfaces
```go
// Status enumerates push outcomes.
type Status int
const (
    StatusAccepted Status = iota
    StatusDuplicate
    StatusLateDrop
    StatusMapperDegraded
)

type Buffer interface {
    Push(pkt Packet, mapperState clock.MapperState) Status
    Pop(now time.Time) (Packet, bool)
    Occupancy() time.Duration
    Target() time.Duration
}
```

`Packet` contains RTP metadata, decoded timestamp, and payload reference. Implementations should be concurrency-safe for single producer / single consumer by default; multi-consumer access requires higher-level synchronization.

## Observability
- Metrics (per SSRC and media type)
  - `media_jitter_target_ms`
  - `media_jitter_occupancy_ms`
  - `media_jitter_late_drops_total`
  - `media_jitter_duplicates_total`
  - `media_jitter_underflows_total`
  - `media_jitter_startup_duration_ms`
- Logs
  - Buffer widening/shrinking events (rate-limited).
  - Underflow warnings with current occupancy and drift state.

## Integration & Dependencies
- Depends on clock mapper for stable timestamps; receives `MapperState` to detect degradation.
- Integrated in `internal/media/buffer/jitter.go` with constructors `NewAudioBuffer(cfg)` and `NewVideoBuffer(cfg)`.
- `internal/pipeline` ensures each subscriber uses buffer `Push`/`Pop` around decoder/mixer.
- `internal/metrics` registers counters/histograms using repo-standard collector helpers.

## Failure Modes & Mitigations
- **Sustained High Jitter**: Buffer clamps at `max_target_ms`; escalate via control plane to downgrade quality or reconnect.
- **Mapper Degradation**: Enter degraded mode, widen buffer, log, and consider resync.
- **Memory Pressure**: Enforce `max_reorder_depth`; if exceeded, drop oldest packets and log `ERROR` with SSRC.
- **Duplicate Flood**: Rate-limit logs; consider upstream mitigation if duplicates exceed threshold.

## Future Work
- Experiment with per-participant latency tiers (active speaker vs passive tiles).
- Integrate forward error correction (FEC) hints to better utilize stored packets.
- Evaluate machine-learning based jitter prediction to pre-emptively adjust target depth.
