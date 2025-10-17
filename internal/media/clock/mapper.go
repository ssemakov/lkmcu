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

func (cm *ClockMapper) UpdateFromSR(ssrc, rtp_ts, ntp_ts uint64) error {
	return nil
}

func (cm *ClockMapper) RTPToNTP(ssrc, rtp_ts uint32) (time.Time, MapperState, error) {
	return 0, MapperState{}, nil
}

func (cm *ClockMapper) State(ssrc uint32) MapperState {
	return MapperState{}
}

func (cm *ClockMapper) ForEach(f func(ssrc uint32, state MapperState) bool) {
}
