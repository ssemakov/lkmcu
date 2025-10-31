package clock

import (
	"math"
	"sync"
	"time"

	"github.com/ssemakov/lkmcu/internal/logging"
	"github.com/ssemakov/lkmcu/internal/metrics"
)

type ssrcDrift struct {
	errs      *RingBuffer[driftSample] // rolling window of recent drift samples
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
type Drift struct {
	metrics *metrics.Metrics
	logger  logging.Logger

	mu sync.Mutex
	dm map[uint32]*ssrcDrift // drift per SSRC

	cfg DriftConfig
}

type correctionAction string

const (
	None           correctionAction = "NoCorrection"
	AdjustSlightly correctionAction = "AdjustSlightly"
	RequestResync  correctionAction = "RequestResync"
)

type DriftAssessment struct {
	currentPpm               float64
	correctionRecommendation correctionAction
	hysteresisApplied        bool
}

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

func NewDriftController(metrics *metrics.Metrics, logger logging.Logger) *Drift {
	return &Drift{
		metrics: metrics,
		logger:  logger,
		dm:      make(map[uint32]*ssrcDrift),
		cfg:     DefaultDriftConfig(),
	}
}

func (d *Drift) Observe(ssrc uint32, expected, arrival time.Time) DriftAssessment {
	d.mu.Lock()
	sd, ok := d.dm[ssrc]
	if !ok {
		sd = &ssrcDrift{
			errs: NewRingBuffer[driftSample](d.cfg.MaxSamples),
			cfg:  &d.cfg,
		}
		d.dm[ssrc] = sd
	}
	d.mu.Unlock()

	errS := arrival.Sub(expected).Seconds()
	sd.errs.Push(driftSample{at: arrival, errS: errS})
	if sd.lastObs.IsZero() {
		sd.lastObs = arrival
		sd.lastErr = errS

		return DriftAssessment{}
	}

	ppm := sd.updateEWMA(arrival, errS)
	assess := sd.assessPPM(ppm)
	d.exportMetrics(assess, ssrc)

	return assess
}

func (d *Drift) Reset(ssrc uint32) {
	d.mu.Lock()
	defer d.mu.Unlock()

	delete(d.dm, ssrc)
}

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
	sd.mu.Lock()
	defer sd.mu.Unlock()

	dt := arrival.Sub(sd.lastObs).Seconds() // elapsed time since last sample
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
	sd.mu.Lock()
	defer sd.mu.Unlock()

	arrival := sd.lastObs
	absPpm := math.Abs(ppm)

	a := DriftAssessment{
		currentPpm: math.Round(ppm),
	}

	if absPpm > sd.cfg.CritPPM {
		if sd.over11ppmSince.IsZero() {
			sd.over11ppmSince = arrival
		}
		if sd.over6ppmSince.IsZero() {
			sd.over6ppmSince = arrival
		}

	} else if absPpm > sd.cfg.WarnPPM {
		if sd.over6ppmSince.IsZero() {
			sd.over6ppmSince = arrival
		}
		sd.over11ppmSince = time.Time{}
	} else {
		sd.over6ppmSince, sd.over11ppmSince = time.Time{}, time.Time{}
		a.correctionRecommendation = None
		return a
	}

	// Decide recommendation with hysteresis
	if !sd.over11ppmSince.IsZero() && arrival.Sub(sd.over11ppmSince) >= sd.cfg.SustainCrit {
		a.correctionRecommendation = RequestResync
		a.hysteresisApplied = true
	}
	if !sd.over6ppmSince.IsZero() && arrival.Sub(sd.over6ppmSince) >= sd.cfg.SustainWarn {
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
