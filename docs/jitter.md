# Jitter Buffer Design

## Purpose

Describe the buffering strategy that smooths RTP packet arrival variance before decoding/mixing while balancing end-to-end latency. This design depends on the master timeline produced by the [RTP→NTP Clock Mapping & Drift Control](./rtp_clock_mapping.md) document.

## Goals

- Provide stable playout for audio/video under typical network jitter using baseline latency of 80–120 ms.
- Keep latency bounded with a fixed nominal target and a widened target while the clock mapper is degraded.
- Handle late and duplicate packets deterministically with clear observability.
- Offer configuration knobs through `jitter.Config`.

## Mental Model

```
                BUFFER WINDOW
           ------------------------------------------------------------
PacketTS: |-------- OLD -------|----- READY -----|------ UPCOMING ----|
          | too-late | dropped | pop() emits     | still waiting      |
          -------------------------------------------------------------
                ^                        ^                  ^
                |                        |                  |
          cursor - late_drop_ms    cursor(now-target)      now
```

Any arriving packet can be

1. Too late:
   packet_pts < cursor - late_drop_ms
   It drop immediately.

2. Ready:
   packet_pts ≤ cursor
   Pop() returns packet.

3. Future packets:
   packet_pts > cursor
   Will be keep in buffer until cursor advances.

## Architecture Overview

1. **Ingress**: Jitter buffer receives RTP packets tagged with master-clock presentation timestamps from the mapper.
2. **Queueing**: Packets stored in per-SSRC heaps indexed by timestamp (secondary key: sequence number).
3. **Playback Controller**: Maintains playout cursor on master clock, releases packets when `packet_pts ≤ cursor`.
4. **Target Control**: Uses the nominal target in steady state and widens to `MaxTarget` when the mapper reports degraded timing.
5. **Integration**: Track pipelines `Pop()` packets aligned to the cursor, then pass them through depacketization and decode.

## Configuration Defaults

| Config field      | Default    | Notes                                           |
| ----------------- | ---------- | ----------------------------------------------- |
| `Target`          | 80 ms      | Nominal playout depth in steady state           |
| `MaxTarget`       | 200 ms     | Depth used while mapper timing is degraded      |
| `LateDrop`        | 20 ms      | Drop if packet lags beyond cursor + threshold   |
| `RebufferBurst`   | 20 ms      | Cursor freeze window when underflow is detected |
| `UndeflowGuard`   | 20 ms      | Empty-buffer duration before underflow handling |
| `MaxReorderDepth` | 50 packets | Maximum retained packet count per SSRC          |
| `DupeBufferSize`  | 100 keys   | Sliding duplicate-detection window per SSRC     |

These values are currently provided by `internal/media/jitter.DefaultConfig()` and passed directly to `NewBuffer`.

## Runtime Behavior

### Startup

- Before the first trustworthy SR arrives, `Push()` may temporarily hold packets using arrival time only.
- `Pop()` does not release packets while the SSRC remains in `noSR` mode.
- On the first SR (`Uninitialized -> Anchored` in the mapper), the buffer flushes all pre-SR packets and starts fresh on the mapper-derived RTP→NTP timeline.
- Startup therefore does not attempt to salvage or remap pre-SR packets; it prefers a short startup loss over mixing arrival-time and anchored-time packets in one heap.

### Target Depth Control

- In steady state, each SSRC uses `Target`.
- If the clock mapper reports degraded timing, that SSRC widens immediately to `MaxTarget`.
- When mapper timing recovers, the SSRC returns to `Target`.
- If occupancy grows beyond the active target, the buffer drops oldest packets until occupancy is within bounds.
- RMS (Root Mean Square) jitter tracking, gradual growth/shrink, and metrics are future work.

### Packet Admission

- `Push(ssrc, packet)` path:
  1. Read mapper state and convert RTP timestamp to NTP time when the mapper is anchored.
  2. If the mapper is uninitialized, hold packets in `noSR` mode using arrival time only; these packets are flushed when the first SR arrives.
  3. Return `StatusMapperError` for mapper conversion errors unless the mapper is degraded/discontinuous and still provides a usable best-effort timestamp.
  4. Drop duplicates using `{presentation_timestamp, sequence}`.
  5. Drop packets older than `cursor - LateDrop`.
  6. Insert into the min-heap keyed by `(presentation_timestamp, sequence)`.
  7. Enforce `MaxReorderDepth` and active target occupancy by dropping oldest packets.

### Playout

- `Pop(ssrc, now)` uses cursor = `now - target`, where `target` is either `Target` or `MaxTarget`.
- Release next packet when heap head `packet_pts ≤ cursor`.
- If the heap is empty for longer than `UndeflowGuard` after the last pop, trigger underflow handling:
  - Freeze cursor advancement for `RebufferBurst`.
  - Invoke the configured `OnUnderflowHandler` once for that underflow window.

### Degraded Mode

- When mapper reports SR outage or extrapolation beyond window, buffer widens `target_ms` to `max_target_ms` immediately.
- Escalation beyond widening/logging is future work.

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
    Push(ssrc uint32, pkt packet.Packet) (Status, error)
    Pop(ssrc uint32, now time.Time) (packet.Packet, bool)
    Occupancy(ssrc uint32) time.Duration
    Target(ssrc uint32) time.Duration
    Config() *Config
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
  - Buffer widening when mapper is degraded.
  - Late drops, memory pressure, and occupancy trims.

Metric integration listed here is planned; `internal/metrics` currently contains placeholders for this area.

## Integration & Dependencies

- Depends on `internal/media/clock/mapper.go` for RTP→NTP timestamping and mapper health.
- Implemented in `internal/media/jitter/buffer.go`.
- Currently wired into the subscriber pipeline on `Push()` from `internal/media/processor/track_pipeline.go`.
- `internal/metrics` integration described here is still aspirational and should be treated as future work unless explicitly implemented.

## Failure Modes & Mitigations

- **Sustained High Jitter**: Buffer clamps occupancy at the active target by dropping oldest packets.
- **Mapper Degradation**: Enter degraded mode, widen buffer, log, and consider resync.
- **Memory Pressure**: Enforce `max_reorder_depth`; if exceeded, drop oldest packets and log `ERROR` with SSRC.
- **Duplicate Flood**: Drop duplicates through the sliding duplicate key buffer; upstream mitigation is future work.

## Future Work

- Experiment with per-participant latency tiers (active speaker vs passive tiles).
- Integrate forward error correction (FEC) hints to better utilize stored packets.
- Evaluate machine-learning based jitter prediction to pre-emptively adjust target depth.
- Add per-media jitter configs and adaptive RMS-based target growth/shrink.
- Export jitter metrics through `internal/metrics`.
