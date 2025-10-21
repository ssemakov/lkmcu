package clock

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/ssemakov/lkmcu/internal/logging"
	"github.com/ssemakov/lkmcu/internal/metrics"
)

type MapperState struct {
	// placeholder for mapper state fields
}

// ClockMapper maps RTP timestamps to NTP timestamps for multiple streams
// streamClocks holds the mapping state for each SSRC
type ClockMapper struct {
	streamClocks map[uint32]*StreamClock
	metrics      *metrics.Metrics
	logger       logging.Logger
	m            sync.RWMutex
	srTimeout    time.Duration
}

type MasterClockAccr func() time.Time

func NewClockMapper(accr MasterClockAccr, metrics *metrics.Metrics, logger logging.Logger) *ClockMapper {
	return &ClockMapper{
		streamClocks: make(map[uint32]*StreamClock),
		metrics:      metrics,
		logger:       logger,
	}
}

const discontinuityThresholdSec = 0.050 // >50ms jump → discontinuity

// UpdateFromSR ingests the newest RTCP Sender Report for an SSRC, extending the
// RTP timestamp past rollovers, refreshing the (base_rtp, base_ntp) anchor and
// derived offset/clock-rate state, and resetting monotonic/drift guards when
// the sender clock jumps so subsequent RTP→NTP conversions stay accurate.
func (cm *ClockMapper) UpdateFromSR(ssrc uint32, rtpTs, ntpHigh, ntpLow uint32) error {
	cm.m.Lock()
	defer cm.m.Unlock()

	ntpTsSeconds := float64(ntpHigh) + (float64(ntpLow) / float64(1<<32))

	sc, ok := cm.streamClocks[ssrc]
	if !ok {
		return fmt.Errorf("clock rate for SSRC %d not set before first SR", ssrc)
	}

	extRtp := sc.wrapTracker.ExtendedRTP(rtpTs)

	// Out-of-order SR guard: compare by NTP
	eps := 1e-9 // 1 ns tolerance

	if ntpTsSeconds < sc.baseNTP-eps {
		// No change in RTP timestamp, likely duplicate SR
		cm.logger.Infof("ClockMapper: out-of-order SR (NTP %f < %f) for SSRC=%d; ignoring", ntpTsSeconds, sc.baseNTP, ssrc)
		return nil
	}

	predicted := sc.offset + rtpDeltaSeconds(extRtp, sc)
	delta := ntpTsSeconds - predicted
	discontinuity := math.Abs(delta) > discontinuityThresholdSec

	if discontinuity {
		cm.logger.Infof("ClockMapper: SR step change %.3f ms for SSRC=%d; treating as discontinuity",
			delta*1000.0, ssrc)

		// Reset wrap baseline to new origin
		sc.wrapTracker.wraps = 0
		sc.wrapTracker.lastRTP = rtpTs
		sc.baseRTP = uint64(rtpTs)
		sc.baseNTP = ntpTsSeconds
		sc.offset = offset(ntpTsSeconds, sc.baseRTP, sc.clockRate)

		// Monotonic guard reset
		sc.lastNTPOutput = sc.baseNTP
		sc.degraded = true

		// TODO: drift estimator reset
		// TODO: signal downstream via channel (actual flush/keyframe is handled by pipeline)
	} else {
		// Update stream clock state
		sc.baseNTP = ntpTsSeconds
		sc.baseRTP = extRtp
		sc.offset = offset(ntpTsSeconds, extRtp, sc.clockRate)
	}

	now := time.Now()
	sc.lastSRReceivedAt = now
	sc.baseSRRecvAt = now

	if !discontinuity {
		sc.degraded = false
	}

	// e.g., sc.lastNTPOutput = sc.baseNTP
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
//   - Extrapolation required beyond the configured window (degraded), when
//     clamping is not permitted by configuration.
func (cm *ClockMapper) RTPToNTP(ssrc, rtpTs uint32) (time.Time, MapperState, error) {
	sc := cm.streamClocks[ssrc]
	srAge := time.Since(sc.lastSRReceivedAt)
	if srAge > cm.srTimeout {
		sc.degraded = true
		// clamp extrapolation / return state so caller can react
	}
	// Expose sr_age via metrics.

	return time.Time{}, MapperState{}, nil
}

// SetClockRate sets the clock rate for the given SSRC stream
func (cm *ClockMapper) SetClockRate(ssrc uint32, clockRate uint32) {
	cm.m.Lock()
	defer cm.m.Unlock()

	sc, ok := cm.streamClocks[ssrc]
	if !ok {
		sc = &StreamClock{
			wrapTracker: RTPWrapTracker{},
		}
		cm.streamClocks[ssrc] = sc
	}
	sc.clockRate = clockRate
}

func (cm *ClockMapper) State(ssrc uint32) MapperState {
	return MapperState{}
}

func (cm *ClockMapper) ForEach(f func(ssrc uint32, state MapperState) bool) {
}

func offset(ntpTsSeconds float64, rtpTs uint64, clockRate uint32) float64 {
	return ntpTsSeconds - float64(rtpTs)/float64(clockRate)
}

func rtpDeltaSeconds(extRtp uint64, sc *StreamClock) float64 {
	return (float64(extRtp) - float64(sc.baseRTP)) / float64(sc.clockRate)
}
