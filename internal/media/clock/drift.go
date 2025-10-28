package clock

import (
	"sync"
	"time"

	"github.com/ssemakov/lkmcu/internal/logging"
	"github.com/ssemakov/lkmcu/internal/metrics"
)

type ssrcDrift struct {
	// errs      ringBuffer[driftSample] // fixed size N
	sumW      float64   // EWMA total weight
	slopeEWMA float64   // seconds error change per second (dimensionless)
	lastObs   time.Time // last arrival time observed
	lastErr   float64   // last err (s)

	// Sustained-state tracking
	over6ppmSince  time.Time
	over11ppmSince time.Time

	// Config snapshot (optional)
	cfg DriftConfig
}

type driftSample struct {
	at   time.Time // monotonic
	errS float64   // seconds (arrival - expected)
}

type DriftConfig struct {
	WindowDuration     time.Duration // e.g., 5 * time.Minute
	MaxSamples         int           // e.g., 50
	EWMAAlpha          float64       // e.g., 0.2
	WarnPPM            float64       // 6
	CritPPM            float64       // 11
	SustainWarn        time.Duration // 30 * time.Second
	SustainCrit        time.Duration // 15 * time.Second (optional)
	MaxAudioPct        float64       // 0.5 (% hard cap)
	VideoNudgePerFrame time.Duration // 2 * time.Millisecond
}
type Drift struct {
	metrics *metrics.Metrics
	logger  *logging.Logger

	mu sync.Mutex
	m  map[uint32]*ssrcDrift // ssrc -> drift info

	cfg DriftConfig
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
	return DriftAssessment{}
}

func (d *Drift) Reset(ssrc uint32) {
}

func (d *Drift) PruneExpired(now time.Time) {
	// placeholder for pruning expired entries
}
