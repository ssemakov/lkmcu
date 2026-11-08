// Package lab runs deterministic experiments using the learner's functions.
package lab

import (
	"fmt"
	"math"
	"time"

	"github.com/ssemakov/lkmcu/labs/audio/exercises"
)

const (
	SampleRate   = 48000
	BlockSamples = 960
	BlockTime    = 20 * time.Millisecond
)

type Config struct {
	Signal     string        `json:"signal"`
	Duration   time.Duration `json:"duration_ns"`
	Frequency  float64       `json:"frequency_hz"`
	Amplitude  float64       `json:"amplitude"`
	ImpulseAt  int           `json:"impulse_at"`
	Gain       *float64      `json:"gain,omitempty"`
	Clip       bool          `json:"clip"`
	Prediction string        `json:"prediction,omitempty"`
	Note       string        `json:"note,omitempty"`
}

// Functions makes runner verification possible with small fixtures, without
// putting reference exercise implementations into the lab.
type Functions struct {
	Sine        func(int, int, float64, float64) []float32
	Sandbox     func(int, int) []float32
	Peak        func([]float32) float64
	RMS         func([]float32) float64
	ApplyGain   func([]float32, float64) []float32
	AmplitudeDB func(float64) float64
	HardClip    func([]float32) []float32
}

func LearnerFunctions() Functions {
	return Functions{
		Sine: exercises.Sine, Sandbox: exercises.Sandbox,
		Peak: exercises.Peak, RMS: exercises.RMS,
		ApplyGain: exercises.ApplyGain, AmplitudeDB: exercises.AmplitudeDB,
		HardClip: exercises.HardClip,
	}
}

type Metric struct {
	State   string   `json:"state"`
	Value   *float64 `json:"value,omitempty"`
	Display string   `json:"display"`
	Detail  string   `json:"detail,omitempty"`
}

type Stage struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Samples   []float32 `json:"samples"`
	Peak      Metric    `json:"peak"`
	RMS       Metric    `json:"rms"`
	PeakDBFS  Metric    `json:"peak_dbfs"`
	OverRange int       `json:"over_range_samples"`
	WAV       string    `json:"wav"`
}

type Experiment struct {
	Version      int     `json:"version"`
	CreatedAt    string  `json:"created_at"`
	Config       Config  `json:"config"`
	SampleRate   int     `json:"sample_rate"`
	BlockSamples int     `json:"block_samples"`
	PlaybackGain float64 `json:"playback_gain"`
	Stages       []Stage `json:"stages"`
}

func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

func (c Config) Validate() error {
	switch c.Signal {
	case "silence", "impulse", "constant", "sine", "sandbox":
	default:
		return fmt.Errorf("signal must be silence, impulse, constant, sine, or sandbox")
	}
	if c.Duration < BlockTime || c.Duration > 10*time.Second || c.Duration%BlockTime != 0 {
		return fmt.Errorf("duration must be a multiple of 20ms, from 20ms to 10s")
	}
	if !finite(c.Amplitude) || c.Amplitude < 0 || c.Amplitude > 16 {
		return fmt.Errorf("amplitude must be finite and between 0 and 16")
	}
	if !finite(c.Frequency) || c.Frequency < 0 || c.Frequency >= SampleRate/2 {
		return fmt.Errorf("frequency must be finite and in [0, 24000) Hz")
	}
	if c.Gain != nil && (!finite(*c.Gain) || math.Abs(*c.Gain) > 16) {
		return fmt.Errorf("gain must be finite and between -16 and 16")
	}
	count := int(c.Duration / BlockTime * BlockSamples)
	if c.Signal == "impulse" && (c.ImpulseAt < 0 || c.ImpulseAt >= count) {
		return fmt.Errorf("impulse position must be in [0, %d)", count)
	}
	return nil
}

func Build(c Config, f Functions) (*Experiment, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	// This is the runner's fixed target preset, not an answer substituted for
	// SamplesForDuration or MixerFormat. Their behavior is checked separately.
	n := int(c.Duration / BlockTime * BlockSamples)
	input, err := callSamples("signal generator", n, func() []float32 {
		switch c.Signal {
		case "sine":
			return f.Sine(SampleRate, n, c.Frequency, c.Amplitude)
		case "sandbox":
			return f.Sandbox(SampleRate, n)
		default:
			x := make([]float32, n)
			if c.Signal == "constant" {
				for i := range x {
					x[i] = float32(c.Amplitude)
				}
			} else if c.Signal == "impulse" {
				x[c.ImpulseAt] = float32(c.Amplitude)
			}
			return x
		}
	})
	if err != nil {
		return nil, err
	}
	e := &Experiment{Version: 1, CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Config: c, SampleRate: SampleRate, BlockSamples: BlockSamples, PlaybackGain: 0.1}
	e.Stages = append(e.Stages, Stage{ID: "input", Name: "Input", Samples: input, WAV: "input.wav"})
	current := input
	if c.Gain != nil {
		current, err = callSamples("ApplyGain", n, func() []float32 {
			return f.ApplyGain(append([]float32(nil), current...), *c.Gain)
		})
		if err != nil {
			return nil, err
		}
		e.Stages = append(e.Stages, Stage{ID: "gained", Name: "After gain", Samples: current, WAV: "gained.wav"})
	}
	if c.Clip {
		current, err = callSamples("HardClip", n, func() []float32 {
			return f.HardClip(append([]float32(nil), current...))
		})
		if err != nil {
			return nil, err
		}
		e.Stages = append(e.Stages, Stage{ID: "clipped", Name: "Hard clipped", Samples: current, WAV: "clipped.wav"})
	}
	maxAbs := 0.0
	for i := range e.Stages {
		s := &e.Stages[i]
		// Give each measurement independent storage so a buggy measurement
		// cannot corrupt either the preserved samples or another measurement.
		s.Peak = measure(func() float64 { return f.Peak(append([]float32(nil), s.Samples...)) }, false)
		s.RMS = measure(func() float64 { return f.RMS(append([]float32(nil), s.Samples...)) }, false)
		s.PeakDBFS = Metric{State: s.Peak.State, Display: s.Peak.Display, Detail: "Requires a valid Peak measurement: " + s.Peak.Detail}
		if s.Peak.State == "ready" {
			s.PeakDBFS = measure(func() float64 { return f.AmplitudeDB(*s.Peak.Value) }, true)
		}
		for _, v := range s.Samples {
			a := math.Abs(float64(v))
			if a > 1 {
				s.OverRange++
			}
			maxAbs = math.Max(maxAbs, a)
		}
	}
	// All stages get the SAME linear listening attenuation. This range scan
	// belongs to WAV export safety, not to the learner's reported Peak metric.
	if maxAbs > 0 {
		e.PlaybackGain = math.Min(e.PlaybackGain, 0.9/maxAbs)
	}
	return e, nil
}

func callSamples(name string, count int, fn func() []float32) (samples []float32, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("%s: %v", name, p)
		}
	}()
	samples = fn()
	if len(samples) != count {
		return nil, fmt.Errorf("%s returned %d samples; want exactly %d", name, len(samples), count)
	}
	for i, v := range samples {
		if !finite(float64(v)) {
			return nil, fmt.Errorf("%s returned non-finite sample at index %d", name, i)
		}
	}
	return samples, nil
}

func measure(fn func() float64, db bool) (m Metric) {
	defer func() {
		if p := recover(); p != nil {
			m = Metric{State: "error", Display: "Error", Detail: fmt.Sprint(p)}
			if _, ok := p.(exercises.Unimplemented); ok {
				m.State, m.Display = "pending", "Pending"
			}
		}
	}()
	v := fn()
	if db && math.IsInf(v, -1) {
		return Metric{State: "ready", Display: "−∞", Detail: "negative infinity"}
	}
	if !finite(v) || (!db && v < 0) {
		return Metric{State: "error", Display: "Invalid", Detail: fmt.Sprintf("function returned %g", v)}
	}
	return Metric{State: "ready", Value: &v, Display: fmt.Sprintf("%.6g", v)}
}
