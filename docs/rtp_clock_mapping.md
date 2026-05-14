# RTP→NTP Clock Mapping & Drift Control Design

## Purpose

Define how the MCU reconstructs a stable wall-clock for incoming RTP streams using RTCP Sender Reports (SR) and how it bounds inter-stream drift to ≤ 40 ms per hour (≈ 11 ppm). This document covers clock-state management, drift detection, correction tactics, and observability. See [Jitter Buffer Design](./jitter.md) for buffering strategy built on top of this timing model.

## Objectives

- Maintain accurate RTP→NTP mappings per SSRC using SR updates and safe extrapolation rules.
- Detect and correct sender clock drift relative to the MCU master clock without audible/visible artifacts.
- Surface configuration levers (timeouts, budgets) and telemetry for operations.

## Clock Model

Each RTP sender periodically emits RTCP SR packets containing a `(rtp_timestamp, ntp_timestamp)` pair. The MCU keeps a per‑SSRC linear mapping anchored at the latest SR:

```
NTP(t_rtp) = base_ntp + (extended_rtp(t_rtp) - base_rtp) / clock_rate
```

Where:

- `base_rtp`, `base_ntp`: the most recent SR anchor. `base_rtp` is kept in an extended 64‑bit timeline to handle rollover.
- `extended_rtp(t)`: 64‑bit RTP timestamp obtained by tracking 32‑bit wraps (see below).
- `clock_rate`: codec‑specific rate (e.g., video 90 kHz, audio 48 kHz); must be set per SSRC.

Here's how RTP maps to NTP via base values from the latest SR.

```
Time (in seconds) ─────────────────────────────────────────────────────────────►

       ┌──────────────────────────────────────────────────────────┐
       │                        NTP Domain                        │
       └──────────────────────────────────────────────────────────┘
       base_ntp (from SR)
          │
          │
          │         +1s later                        +2s later
          │              │                                 │
          ▼              ▼                                 ▼
     ┌────────────┐   ┌────────────┐                 ┌────────────┐
     │ NTP=1000.0 │ → │ NTP=1001.0 │ ... (linear) ...│ NTP=1002.0 │
     └────────────┘   └────────────┘                 └────────────┘
          ▲
          │
          │   linear relation:
          │   NTP(t_rtp) = base_ntp + (t_rtp - base_rtp) / clock_rate
          │
          │
       ┌──────────────────────────────────────────────────────────┐
       │                        RTP Domain                        │
       └──────────────────────────────────────────────────────────┘
       base_rtp (from SR)
          │
          │
          │         +90,000 ticks later               +180,000 ticks later
          │              │                                   │
          ▼              ▼                                   ▼
     ┌────────────┐   ┌────────────┐                 ┌────────────┐
     │ RTP=0      │ → │ RTP=90000  │ ... (linear) ...│ RTP=180000 │
     └────────────┘   └────────────┘                 └────────────┘
          ▲
          │
          │  RTCP SR ties these two worlds together:
          │  (base_rtp=0) ↔ (base_ntp=1000.0)
          │
          └────────────── mapping anchor (Sender Report)
```

### Intuition

- `base_rtp` and `base_ntp` define a tie-point between RTP tick space and wall-clock seconds.
- Every new RTP timestamp can be **linearly projected** forward using the codec’s sample rate.
- This keeps all tracks aligned to a global time domain (NTP) for mixing, synchronization, and drift estimation.

### Per-SSRC State

- `base_rtp` (uint64), `base_ntp` (float64 seconds)
- `clock_rate` (uint32)
- `wrap_tracker` (last_rtp, wraps) to extend 32‑bit RTP into 64‑bit space
- `last_ntp_out` (float64): monotonic guard for converted timestamps
- `last_sr_received_at` (time.Time): arrival time used for SR freshness
- `state`: explicit clock phase
  - `Uninitialized`: no SR has anchored the stream yet
  - `Anchored`: first SR installed; RTP→NTP mapping is usable for new packets
  - `Locked`: a later consistent SR confirmed continuity
- `degraded`, `discontinuity` flags for observability/guards

### Update Flow

1. **SR Arrival**
   - Out‑of‑order guard: SRs with NTP older than the current anchor (within an `epsilon` tolerance) are ignored.
   - State transition on accepted SR:
     - `Uninitialized -> Anchored`: first SR establishes the initial RTP→NTP anchor and does not count as a discontinuity.
     - `Anchored -> Locked`: second accepted SR confirms continuity and moves the stream into steady state.
     - `Locked -> Locked`: later accepted SRs refresh the anchor in steady state.
   - On accepted non-bootstrap SRs, extend RTP to 64‑bit via wrap detection and compare the SR NTP to the predicted NTP from the current model.
   - If `|delta| > discontinuity_threshold`, treat the SR as a discontinuity:
     - Reset wrap baseline (`wraps=0`, `last_rtp=rtp_new`).
     - Set `base_rtp = rtp_new`, `base_ntp = sr.ntp`.
     - Reset `last_ntp_out = base_ntp`; set `degraded = true`, `discontinuity = true`.
   - Otherwise, refresh `base_rtp`, `base_ntp`, and `last_sr_received_at`, and clear `degraded`/`discontinuity`.
2. **Between SRs**
   - Convert incoming RTP timestamps using the linear model with the wrap tracker.
   - `Uninitialized` streams reject conversion with a no-SR error.
   - `Anchored` and `Locked` streams both allow mapping of new packets.
   - Enforce monotonic output per SSRC. If the computed NTP would move backwards relative to `last_ntp_out`:
     - If the backstep is small (`< backstep_clamp_threshold`), clamp forward to `last_ntp_out + clamp_smoothing`.
     - Otherwise, mark `degraded = true`, `discontinuity = true`, and clamp to `last_ntp_out + clamp_smoothing`.
3. **Missing SRs**
   - After `sr_timeout` without a fresh accepted SR, mark the mapping `degraded` and return an error on conversion while still providing a best-effort projected NTP.

## Drift Detection & Correction

### Measurement

- Compare `expected_time = mapper.RTPToNTP(rtp_ts)` with the MCU master clock (`arrival_time`) after compensating for network RTT if available.
- Maintain sliding window (e.g., 5 min, 50 samples) of `(expected_time, arrival_time)` deltas.
- Use EWMA (Exponentially Weighted Moving Average) or linear regression to estimate drift slope; reset estimator on SR discontinuities:

Formula:
EWMA(t) = α · slope + (1 − α) · EWMA(t−1)
slope = (arrival_time - expected_time) / elapsed_time_since_last_sample

Where:
α = smoothing factor (0 < α ≤ 1)
→ High α = reacts quickly (less smoothing)
→ Low α = reacts slowly (more smoothing)

### Drift Budget & Thresholds

- Target: |drift| ≤ 40 ms/hour (≈ 11 ppm).
- Warning threshold: ±6 ppm sustained for > 30 s → schedule corrective action.
- Critical threshold: ±11 ppm sustained → escalate (request keyframe, resubscribe, notify control plane).

### Correction Strategy

- **Audio**
  - Drive fractional resampler within ±0.5 % envelope to gradually align clocks.
  - For residual spikes, insert/drop a single 10 ms frame per ≥ 2 min, cross-fading edges to avoid clicks.
- **Video**
  - Nudge presentation timestamps toward master clock (≤ 2 ms per frame adjustment).
  - Drop or duplicate frames only when drift > 20 ms and resync not imminent; prefer keyframe request if divergence persists.
- **Reset Conditions**
  - If SR arrives with backwards NTP jump > 5 ms, treat as sender clock reset: flush jitter buffer, reset drift estimator.

## Observability & Config

- Metrics
  - `media_clock_sr_age_seconds{ssrc}`
  - `media_clock_drift_ppm{ssrc}`
  - `media_clock_corrections_total{type}`
  - `media_clock_correction_duration_ms`
- Logs
  - Rate-limited warnings when drift crosses warning threshold for > 30 s.
  - Info logs when SR timeout enters/exits degraded mode.
- Config knobs (`internal/config`)
  - `media.clock.sr_timeout`
  - `media.clock.max_extrapolation_window`
  - `media.clock.drift_budget_ms_per_hr`
  - `media.clock.drift_warning_ppm`

## Integration Points

- `internal/media/clock/mapper.go`: implements the RTP→NTP mapping and SR state machine above.
- `internal/media/clock/stream_clock.go`: owns the per-SSRC clock phase and anchor/discontinuity helpers.
- `internal/media/clock/drift.go`: contains the drift estimator logic.
- `internal/media/jitter/buffer.go`: consumes mapper output for packet admission and degraded-mode widening.
- `internal/pipeline` / `internal/media/processor`: inject mapper and jitter buffer into subscriber lifecycle.

## Failure Modes & Mitigations

- **SR Loss**: enter degraded mode, widen jitter buffer, raise alert via metrics; optionally request peer connection health check.
- **Clock Jump**: detect via large negative/positive convert deltas; reset state and flush buffers to avoid corruption.
- **Mapper Overflow**: guard 64-bit NTP computations; clamp outputs within `time.Duration` limits.

## Future Work

- Incorporate Absolute Capture Time RTP header extension when available to reduce SR dependence.
- Evaluate PTP grandmaster distribution across MCU nodes for tighter cross-node alignment.
- Feed drift metrics into adaptive simulcast layer selection to shed problematic senders.
