package clock

import (
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
	logger       *logging.Logger
	m            sync.RWMutex
}

type MasterClockAccr func() time.Time

func NewClockMapper(accr MasterClockAccr, metrics *metrics.Metrics, logger *logging.Logger) *ClockMapper {
	return &ClockMapper{
		streamClocks: make(map[uint32]*StreamClock),
		metrics:      metrics,
		logger:       logger,
	}
}

// UpdateFromSR ingests the newest RTCP Sender Report for an SSRC, extending the
// RTP timestamp past rollovers, refreshing the (base_rtp, base_ntp) anchor and
// derived offset/clock-rate state, and resetting monotonic/drift guards when
// the sender clock jumps so subsequent RTP→NTP conversions stay accurate.
func (cm *ClockMapper) UpdateFromSR(ssrc, rtp_ts, ntp_ts uint32) error {
	cm.m.Lock()
	defer cm.m.Unlock()

	sc, ok := cm.streamClocks[uint32(ssrc)]
	if !ok {
		sc = &StreamClock{
			baseRTP:   uint64(rtp_ts),
			baseNTP:   uint64(ntp_ts),
			clockRate: 48000, // TODO: default to audio clock rate; should be set properly elsewhere
			offset:    offset(ntp_ts, rtp_ts, 48000),
			wrapTracker: RTPWrapTracker{
				wraps: 0, // zero wraps initially
			},
		}
		cm.streamClocks[uint32(ssrc)] = sc
		return nil
	}

	// Calculate extended RTP timestamp considering wraparounds
	if rtp_ts < uint32(sc.baseRTP) && (sc.baseRTP-uint64(rtp_ts)) > (1<<31) {
		sc.wrapTracker.wraps++
	}

	// extended monotonic RTP timestamp
	extRtp := uint64(rtp_ts) + uint64(sc.wrapTracker.wraps)*(2^32)
	sc.baseNTP = uint64(ntp_ts)
	sc.baseRTP = extRtp
	sc.offset = offset(ntp_ts, rtp_ts, sc.clockRate)

	// Reset monotonic guard and drift estimator if SR indicates sender clock reset.s

	return nil
}

func offset(ntp_ts, rtp_ts, clockRate uint32) int64 {
	return int64(ntp_ts) - int64(rtp_ts)/int64(clockRate)
}

// RTPToNTP converts a packet's 32-bit RTP timestamp for the given SSRC into an
// absolute wall-clock time using the most recent mapping derived from RTCP SRs.
// It first extends the 32-bit RTP value across rollovers using the stream's
// wrap tracker, then applies the linear model anchored at the last SR:
//   NTP = offset + (extended_rtp - base_rtp) / clock_rate
//
// Returns the converted time and a snapshot of the mapper state for
// observability (e.g., SR age, extrapolation window usage). The output is kept
// monotonic per SSRC; if the computed value would move backwards relative to
// the last output, it is clamped/adjusted forward. This function does not apply
// network-arrival or RTT compensation; consumers (e.g., drift controller)
// compare the expected time to the MCU master clock separately.
//
// Errors:
// - Mapping uninitialized (no valid SR seen for SSRC).
// - Extrapolation required beyond the configured window (degraded), when
//   clamping is not permitted by configuration.
func (cm *ClockMapper) RTPToNTP(ssrc, rtp_ts uint32) (time.Time, MapperState, error) {
	sc, ok := cm.streamClocks[ssrc]

	if !ok {
		return time.Time{}, MapperState{}, nil
	}

}

func (cm *ClockMapper) State(ssrc uint32) MapperState {
	return MapperState{}
}

func (cm *ClockMapper) ForEach(f func(ssrc uint32, state MapperState) bool) {
}
