package clock

import (
	"math"
	"sync"
	"time"

	"github.com/ssemakov/lkmcu/internal/logging"
	"github.com/ssemakov/lkmcu/internal/metrics"
)

type ssrcDrift struct {
	samples   *RingBuffer[driftSample] // rolling window of recent drift samples
	slopeEWMA float64                  // seconds error change per second (dimensionless)
	lastObs   time.Time                // last arrival time observed
	lastErr   float64                  // last error observed (seconds)

	// Sustained-state tracking
	over6ppmSince  time.Time // time when drift exceeded 6ppm
	over11ppmSince time.Time // time when drift exceeded 11ppm

	mu  sync.Mutex
	cfg *DriftConfig // reference of config for easy access
}

// Single drift measurement
type driftSample struct {
	at   time.Time // monotonic, when sample was taken
	errS float64   // seconds (arrival - expected), positive means packets arrive late (sender is slow), negative means packets arrive early (sender is fast)
}

// driftSampleEquals is used to compare two driftSample values for equality.
func driftSampleEquals(a, b driftSample) bool {
	return a.at == b.at
}

// DriftConfig captures the thresholds and smoothing constants used while
// estimating the difference between sender and receiver clocks.
//
// The defaults returned by DefaultDriftConfig serve as a safe starting point;
// override selectively if tighter or looser hysteresis is required for a
// deployment.
type DriftConfig struct {
	WindowDuration     time.Duration // duration of rolling window for drift samples
	MaxSamples         int           // cap on number of samples in rolling window
	EWMAAlpha          float64       // Smoothing factor α for the exponentially weighted moving average (0 < α ≤ 1). Lower = smoother, slower reaction.
	WarnPPM            float64       // warning drift threshold in parts per million, When sustained triggers correction
	CritPPM            float64       // critical drift threshold in parts per million, When sustained triggers hard correction (keyframe request / resync)
	SustainWarn        time.Duration // min duration drift must exceed WarnPPM to be considered sustained
	SustainCrit        time.Duration // min duration drift must exceed CritPPM to be considered sustained
	MaxAudioPct        float64       // hard cap on audio sample rate adjustment percentage (e.g., 0.02 = ±2%). Prevents excessive audio pitch shift.
	VideoNudgePerFrame time.Duration // Max timestamp nudge per video frame when correcting video pacing
}

// Drift tracks per-SSRC clock drift and recommends corrective action once
// jitter-free drift surpasses configured thresholds.
//
// It is safe for concurrent use. Each SSRC is guarded by its own mutex so
// independent streams do not block each other.
type Drift struct {
	metrics *metrics.Metrics
	logger  logging.Logger

	mu sync.Mutex
	dm map[uint32]*ssrcDrift // drift per SSRC

	cfg DriftConfig
}

type correctionAction string

const (
	// None means no correction is required beyond normal pacing.
	None correctionAction = "NoCorrection"
	// AdjustSlightly requests a gentle correction (e.g. small sample-rate nudge).
	AdjustSlightly correctionAction = "AdjustSlightly"
	// RequestResync escalates to a hard correction such as a keyframe request.
	RequestResync correctionAction = "RequestResync"
)

// DriftAssessment summarizes the current estimated drift and any recommended
// corrective action after considering hysteresis.
type DriftAssessment struct {
	currentPpm               float64
	correctionRecommendation correctionAction
	hysteresisApplied        bool
}

// DefaultDriftConfig returns a configuration that balances responsiveness with
// tolerance for short-term jitter. Use this as a template when tuning per
// workload.
func DefaultDriftConfig() DriftConfig {
	return DriftConfig{
		WindowDuration:     5 * time.Minute,
		MaxSamples:         100,
		EWMAAlpha:          0.1,
		WarnPPM:            6.0,
		CritPPM:            11.0,
		SustainWarn:        30 * time.Second,
		SustainCrit:        15 * time.Second,
		MaxAudioPct:        0.02,                   // ±2%
		VideoNudgePerFrame: 500 * time.Microsecond, // 0.5ms per frame
	}
}

// NewDriftController builds a Drift tracker hooked up to metrics and logging.
//
// Metrics may be nil if exporting measurements is not required.
func NewDriftController(metrics *metrics.Metrics, logger logging.Logger) *Drift {
	return &Drift{
		metrics: metrics,
		logger:  logger,
		dm:      make(map[uint32]*ssrcDrift),
		cfg:     DefaultDriftConfig(),
	}
}

// Observe ingests the latest packet arrival for the provided SSRC, updating the
// drift model and returning the current assessment.
//
// Thread safety: the method is safe for concurrent use. Updates for the same
// SSRC are serialized internally so callers do not need to coordinate.
func (d *Drift) Observe(ssrc uint32, expected, arrival time.Time) DriftAssessment {
	d.mu.Lock()
	sd, ok := d.dm[ssrc]
	if !ok {
		sd = &ssrcDrift{
			samples: NewRingBuffer[driftSample](d.cfg.MaxSamples, driftSampleEquals),
			cfg:     &d.cfg,
		}
		d.dm[ssrc] = sd
	}
	d.mu.Unlock()

	errS := arrival.Sub(expected).Seconds()
	sd.mu.Lock()
	defer sd.mu.Unlock()

	sd.samples.Push(driftSample{at: arrival, errS: errS})
	if sd.lastObs.IsZero() {
		sd.lastObs = arrival
		sd.lastErr = errS

		// short-circuit until we have at least two samples to compute slope
		return DriftAssessment{
			currentPpm:               0,
			correctionRecommendation: None,
		}
	}

	ppm := sd.updateEWMA(arrival, errS)
	assess := sd.assessPPM(ppm)
	d.exportMetrics(assess, ssrc)

	return assess
}

// Reset discards any drift history for the supplied SSRC, typically when a
// stream is torn down or reinitialized.
func (d *Drift) Reset(ssrc uint32) {
	d.mu.Lock()
	defer d.mu.Unlock()

	delete(d.dm, ssrc)
}

// PruneExpired removes per-SSRC state that has not been updated within the
// configured window, preventing stale entries from accumulating indefinitely.
func (d *Drift) PruneExpired(now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for ssrc, sd := range d.dm {
		if now.Sub(sd.lastObs) > d.cfg.WindowDuration {
			delete(d.dm, ssrc)
		}
	}
}

func (sd *ssrcDrift) updateEWMA(arrival time.Time, errS float64) float64 {
	// thread safety: caller holds sd.mu

	dt := arrival.Sub(sd.lastObs).Seconds() // elapsed time since last sample

	// same arrival time, skip and return current ppm
	if dt == 0.0 {
		return sd.slopeEWMA * 1e6 // parts per million
	}

	slope := (errS - sd.lastErr) / dt
	alpha := sd.cfg.EWMAAlpha
	// EWMA(t) = α · x_t + (1 − α) · EWMA(t−1)
	sd.slopeEWMA = alpha*slope + (1-alpha)*sd.slopeEWMA
	sd.lastObs = arrival
	sd.lastErr = errS

	ppm := sd.slopeEWMA * 1e6 // convert to parts per million
	return ppm
}

func (sd *ssrcDrift) assessPPM(ppm float64) DriftAssessment {
	// thread safety: caller holds sd.mu

	arrival := sd.lastObs
	absPpm := math.Abs(ppm)

	a := DriftAssessment{
		currentPpm:               math.Round(ppm),
		correctionRecommendation: None,
	}

	switch {
	case absPpm > sd.cfg.CritPPM:
		if sd.over11ppmSince.IsZero() {
			sd.over11ppmSince = arrival
		}
		if sd.over6ppmSince.IsZero() {
			sd.over6ppmSince = arrival
		}
	case absPpm > sd.cfg.WarnPPM:
		if sd.over6ppmSince.IsZero() {
			sd.over6ppmSince = arrival
		}
		sd.over11ppmSince = time.Time{}
	default:
		sd.over6ppmSince, sd.over11ppmSince = time.Time{}, time.Time{}
		return a
	}

	// Decide recommendation with hysteresis
	if !sd.over11ppmSince.IsZero() && arrival.Sub(sd.over11ppmSince) >= sd.cfg.SustainCrit {
		a.correctionRecommendation = RequestResync
		a.hysteresisApplied = true
	} else if !sd.over6ppmSince.IsZero() && arrival.Sub(sd.over6ppmSince) >= sd.cfg.SustainWarn {
		a.correctionRecommendation = AdjustSlightly
		a.hysteresisApplied = true
	}

	return a
}

func (d *Drift) exportMetrics(assess DriftAssessment, ssrc uint32) {
	// TODO: implement metric export
	// labels := map[string]string{"ssrc": string(ssrc)}
	// m.SetGauge("media_clock_drift_ppm", labels, sd.slopeEWMA*1e6)

	if assess.correctionRecommendation != None {
		d.logger.Infof("Clock drift for SSRC %08x: %.2f ppm, recommendation: %s (hysteresis applied: %v)",
			ssrc, assess.currentPpm, assess.correctionRecommendation, assess.hysteresisApplied)
	} else {
		d.logger.Debugf("Clock drift for SSRC %08x: %.2f ppm, within acceptable limits", ssrc, assess.currentPpm)
	}
}
