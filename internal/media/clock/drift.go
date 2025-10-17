package clock

import (
	"time"

	"github.com/ssemakov/lkmcu/internal/logging"
	"github.com/ssemakov/lkmcu/internal/metrics"
)

type Drift struct {
	metrics *metrics.Metrics
	logger  *logging.Logger
	// placeholder for drift controller fields
}

type correctionAction string

const (
	None                   correctionAction = "NoCorrection"
	NudgeAudioSampleFactor correctionAction = "NudgeAudioSampleFactor"
	AdjustVideoPacing      correctionAction = "AdjustVideoPacing"
	requestResync          correctionAction = "RequestResync"
)

type DriftAssessment struct {
	currentPpm               int
	correctionRecommendation correctionAction
	hysteresisApplied        bool
}

func NewDriftController(metrics *metrics.Metrics, logger *logging.Logger) *Drift {
	return &Drift{
		metrics: metrics,
		logger:  logger,
	}
}

func (d *Drift) Observe(ssrc uint32, expected, arrival time.Time) DriftAssessment {
	// placeholder for observing drift
	return DriftAssessment{}
}

func (d *Drift) Reset(ssrc uint32) {
}

func (d *Drift) PruneExpired(now time.Time) {
	// placeholder for pruning expired entries
}
