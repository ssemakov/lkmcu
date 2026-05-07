// Package clock maps RTP media timestamps to wall-clock time using RTCP Sender
// Reports and per-stream clock state.
package clock

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/ssemakov/lkmcu/internal/logging"
	"github.com/ssemakov/lkmcu/internal/metrics"
)

// type

// MapperState returned by RTPToNTP for observability
type MapperState struct {
	SRAge            time.Duration
	BaseNTP          float64
	BaseRTP          uint64
	ClockRate        uint32
	LastNTPOut       float64
	Degraded         bool      // indicates if mapping is degraded, e.g., due to SR timeout
	Discontinuity    bool      // indicates if last SR caused a discontinuity
	HasSenderReport  bool      // indicates if mapping has been initialized (at least one SR received)
	LastSRReceivedAt time.Time // timestamp of the last received SR
	Clock            ClockState
}

type ClockMapperConfig struct {
	SRTimeout              time.Duration // duration after which lack of SRs degrades mapping quality
	DiscontinuityThreshold time.Duration // threshold for detecting discontinuities in SRs
	BackstepClampThreshold time.Duration // max allowed backward jump in NTP output before clamping
	ClampSmoothing         time.Duration // amount to smooth NTP clamping by
	Epsilon                time.Duration // tolerance for out-of-order SRs, and NTP output clamping
}

// ClockMapper maps RTP timestamps to NTP timestamps for multiple streams
// streamClocks holds the mapping state for each SSRC
type ClockMapper struct {
	streamClocks map[uint32]*StreamClock
	metrics      *metrics.Metrics
	logger       logging.Logger
	cfg          ClockMapperConfig
	m            sync.RWMutex
}

// ErrNoSenderReport is a sentinel error indicating that no RTCP Sender Report
// has been observed for the given SSRC yet. Use errors.Is(err, ErrNoSenderReport)
// to detect this condition.
var ErrNoSenderReport = errors.New("no sender report")

// NoSenderReportError carries the SSRC for which no Sender Report has been seen.
// It unwraps to ErrNoSenderReport so callers can use errors.Is to detect the
// condition while still retaining SSRC detail via errors.As.
type NoSenderReportError struct {
	SSRC uint32
}

func (e NoSenderReportError) Error() string {
	return fmt.Sprintf("no SR seen yet for SSRC %d", e.SSRC)
}

func (e NoSenderReportError) Unwrap() error { return ErrNoSenderReport }

func DefaultClockMapperConfig() *ClockMapperConfig {
	return &ClockMapperConfig{
		SRTimeout:              5 * time.Second,
		DiscontinuityThreshold: 50 * time.Millisecond,
		BackstepClampThreshold: 10 * time.Millisecond,
		ClampSmoothing:         2 * time.Millisecond,
		Epsilon:                1 * time.Nanosecond,
	}
}

func NewClockMapper(metrics *metrics.Metrics, logger logging.Logger, config *ClockMapperConfig) *ClockMapper {
	if config == nil {
		config = DefaultClockMapperConfig()
	}

	return &ClockMapper{
		streamClocks: make(map[uint32]*StreamClock),
		metrics:      metrics,
		logger:       logger,
		cfg:          *config,
	}
}

// UpdateFromSR ingests the newest RTCP Sender Report for an SSRC, extending the
// RTP timestamp past rollovers, refreshing the (base_rtp, base_ntp) anchor and
// derived offset/clock-rate state, and resetting monotonic/drift guards when
// the sender clock jumps so subsequent RTP→NTP conversions stay accurate.
func (cm *ClockMapper) UpdateFromSR(ssrc uint32, rtpTS, ntpHigh, ntpLow uint32) error {
	cm.m.RLock()
	sc, ok := cm.streamClocks[ssrc]
	cm.m.RUnlock()

	if !ok {
		return fmt.Errorf("clock rate for SSRC %d not set before first SR", ssrc)
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()

	ntpTSSeconds := float64(ntpHigh) + (float64(ntpLow) / float64(1<<32))

	// Out-of-order SR guard: compare by NTP
	// TODO: This out-of-order SR guard skips any SR with an older NTP than baseNTP,
	// even if its RTP timestamp differs. For now we assume reordering is rare and treat
	// these as duplicates. In a future version, check both NTP and RTP equality to
	// distinguish true duplicates from re-ordered or inconsistent SRs.
	if ntpTSSeconds < sc.baseNTP-cm.cfg.Epsilon.Seconds() {
		cm.logger.Debugf("ClockMapper: out-of-order SR (older NTP %f < %f) for SSRC=%d; ignoring",
			ntpTSSeconds, sc.baseNTP, ssrc)
		return nil
	}

	switch sc.state {
	case Uninitialized:
		cm.logger.Debugf("ClockMapper: anchoring clock for SSRC=%08x with first SR (RTP=%d, NTP=%.6f)", ssrc, rtpTS, ntpTSSeconds)
		// anchor the first SR, establishing the initial RTP→NTP mapping, don't degrade or discontinue on first SR
		sc.Anchor(rtpTS, ntpTSSeconds)
	case Anchored:
		cm.logger.Debugf("ClockMapper: locking clock for SSRC=%08x with second SR (RTP=%d, NTP=%.6f)", ssrc, rtpTS, ntpTSSeconds)
		// lock the clock on the second SR, confirming stability of the mapping
		sc.Lock()
		fallthrough
	default:
		sc.Update(rtpTS, ntpTSSeconds, cm.cfg.DiscontinuityThreshold)
	}

	return nil
}

// RTPToNTP converts a packet's 32-bit RTP timestamp for the given SSRC into an
// absolute wall-clock time using the most recent mapping derived from RTCP SRs.
// It first extends the 32-bit RTP value across rollovers using the stream's
// wrap tracker, then applies the linear model anchored at the last SR:
//
//	NTP = offset + (extended_rtp - base_rtp) / clock_rate
//
// Returns the converted time and a snapshot of the mapper state for
// observability (e.g., SR age, extrapolation window usage). The output is kept
// monotonic per SSRC; if the computed value would move backwards relative to
// the last output, it is clamped/adjusted forward. This function does not apply
// network-arrival or RTT compensation; consumers (e.g., drift controller)
// compare the expected time to the MCU master clock separately.
//
// Errors:
//   - Mapping uninitialized (no valid SR seen for SSRC).
//   - TODO: Extrapolation required beyond the configured window (degraded), when
//     clamping is not permitted by configuration.
func (cm *ClockMapper) RTPToNTP(ssrc, rtpTS uint32) (float64, MapperState, error) {
	cm.m.RLock()
	sc, ok := cm.streamClocks[ssrc]
	cm.m.RUnlock()

	if !ok {
		return 0, MapperState{}, fmt.Errorf("clock rate for SSRC %d not set before RTP→NTP", ssrc)
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.lastSRReceivedAt.IsZero() {
		return 0, MapperState{}, NoSenderReportError{SSRC: ssrc}
	}

	// predicted NTP
	ntp := sc.NtpTime(rtpTS)
	srAge := time.Since(sc.lastSRReceivedAt)

	state := MapperState{
		SRAge:            srAge,
		Degraded:         sc.degraded,
		BaseNTP:          sc.baseNTP,
		BaseRTP:          sc.baseRTP,
		ClockRate:        sc.clockRate,
		LastNTPOut:       sc.lastNTPOutput,
		LastSRReceivedAt: sc.lastSRReceivedAt,
		Clock:            sc.state,
	}

	// SR age check
	if srAge > cm.cfg.SRTimeout {
		cm.logger.Debugf("ClockMapper: SR timeout %.3f s for SSRC=%08x; mapping degraded", srAge.Seconds(), ssrc)

		sc.lastNTPOutput = ntp
		sc.degraded = true

		state.LastNTPOut = ntp
		state.Degraded = true

		return ntp, state, fmt.Errorf("SR timeout %.3f s", srAge.Seconds())
	}

	// Monotonic output guard
	if sc.lastNTPOutput != 0 && ntp+cm.cfg.Epsilon.Seconds() < sc.lastNTPOutput {
		backstep := sc.lastNTPOutput - ntp

		if backstep < cm.cfg.BackstepClampThreshold.Seconds() {
			// clamp forward slightly
			ntp = sc.lastNTPOutput + cm.cfg.ClampSmoothing.Seconds()

			deltaMs := (backstep + cm.cfg.ClampSmoothing.Seconds()) * 1000.0
			cm.logger.Debugf("ClockMapper: clamping backward NTP output by %.3f ms (lastNTPOutput=%.6fs, backstep=%.3f ms, smoothing=%.3f ms) for SSRC=%08x",
				deltaMs,
				sc.lastNTPOutput,
				backstep*1000.0,
				cm.cfg.ClampSmoothing.Seconds()*1000.0,
				ssrc,
			)
		} else {
			cm.logger.Debugf("ClockMapper: large backward NTP jump %.3f ms for SSRC=%08x; treating as discontinuity",
				backstep*1000.0, ssrc)

			sc.degraded = true
			sc.discontinuity = true

			// apply smoothing to avoid backstep
			ntp = sc.lastNTPOutput + cm.cfg.ClampSmoothing.Seconds()
			sc.lastNTPOutput = ntp

			state.LastNTPOut = ntp
			state.Degraded = true
			state.Discontinuity = true

			return ntp, state, fmt.Errorf("backstep > %.3f ms", backstep*1000.0)
		}
	}

	sc.lastNTPOutput = ntp
	state.LastNTPOut = sc.lastNTPOutput

	return ntp, state, nil
}

// RTPToNTPTime same as RTPToNTP but returns Unix time
func (cm *ClockMapper) RTPToNTPTime(ssrc, rtpTS uint32) (time.Time, MapperState, error) {
	ntpFloat, state, err := cm.RTPToNTP(ssrc, rtpTS)
	if err != nil && ntpFloat == 0 {
		return time.Time{}, state, err
	}

	const ntpEpochOffsetSeconds = 2208988800 // seconds between 1900-01-01 and 1970-01-01
	unix := ntpFloat - ntpEpochOffsetSeconds

	if unix < 0 {
		return time.Time{}, state, fmt.Errorf("NTP time before Unix epoch for SSRC %d", ssrc)
	}

	sec, frac := math.Modf(unix)
	nsec := frac * 1e9

	return time.Unix(int64(sec), int64(math.Round(nsec))), state, err
}

// SetClockRate sets the clock rate for the given SSRC stream
func (cm *ClockMapper) SetClockRate(ssrc uint32, clockRate uint32) {
	cm.m.Lock()
	sc, ok := cm.streamClocks[ssrc]

	if !ok {
		sc = &StreamClock{
			wrapTracker: RTPWrapTracker{},
		}
		cm.streamClocks[ssrc] = sc
	}
	cm.m.Unlock()

	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.clockRate = clockRate
}

func (cm *ClockMapper) State(ssrc uint32) MapperState {
	cm.m.RLock()
	sc, ok := cm.streamClocks[ssrc]
	cm.m.RUnlock()

	if !ok {
		return MapperState{}
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()

	srAge := time.Duration(0)
	init := !sc.lastSRReceivedAt.IsZero()

	if init {
		srAge = time.Since(sc.lastSRReceivedAt)
	}

	return MapperState{
		SRAge:            srAge,
		BaseNTP:          sc.baseNTP,
		BaseRTP:          sc.baseRTP,
		ClockRate:        sc.clockRate,
		LastNTPOut:       sc.lastNTPOutput,
		Degraded:         sc.degraded,
		Discontinuity:    sc.discontinuity,
		HasSenderReport:  init,
		LastSRReceivedAt: sc.lastSRReceivedAt,
		Clock:            sc.state,
	}
}

func (cm *ClockMapper) ForEach(f func(ssrc uint32, state MapperState) bool) {
	cm.m.RLock()

	states := make(map[uint32]MapperState)

	for ssrc, sc := range cm.streamClocks {
		sc.mu.Lock()

		state := MapperState{
			SRAge: func() time.Duration {
				if sc.lastSRReceivedAt.IsZero() {
					return 0
				} else {
					return time.Since(sc.lastSRReceivedAt)
				}
			}(),
			BaseNTP:          sc.baseNTP,
			BaseRTP:          sc.baseRTP,
			ClockRate:        sc.clockRate,
			LastNTPOut:       sc.lastNTPOutput,
			Degraded:         sc.degraded,
			Discontinuity:    sc.discontinuity,
			LastSRReceivedAt: sc.lastSRReceivedAt,
			HasSenderReport:  !sc.lastSRReceivedAt.IsZero(),
		}
		sc.mu.Unlock()
		states[ssrc] = state
	}
	cm.m.RUnlock()

	for ssrc, state := range states {
		if !f(ssrc, state) {
			break
		}
	}
}
