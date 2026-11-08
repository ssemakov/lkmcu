package lab

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ssemakov/lkmcu/labs/audio/exercises"
)

// These are runner fixtures, not solutions. Infrastructure tests must continue
// working as the learner implements (or temporarily breaks) exercise functions.
func unfinishedFunctions() Functions {
	pendingMetric := func([]float32) float64 { panic(exercises.Unimplemented{Lesson: 3, Function: "fixture"}) }
	return Functions{Peak: pendingMetric, RMS: pendingMetric}
}

func defaultConfig(signal string) Config {
	return Config{Signal: signal, Duration: BlockTime, Frequency: 1000, Amplitude: 0.5}
}

func TestSuppliedSignalsHaveExactBlocksAndPendingMetrics(t *testing.T) {
	for _, signal := range []string{"silence", "impulse", "constant"} {
		t.Run(signal, func(t *testing.T) {
			c := defaultConfig(signal)
			c.ImpulseAt = 480
			e, err := Build(c, unfinishedFunctions())
			if err != nil {
				t.Fatal(err)
			}
			s := e.Stages[0]
			if len(s.Samples) != 960 || s.Peak.State != "pending" || s.RMS.State != "pending" || s.PeakDBFS.State != "pending" {
				t.Fatalf("unexpected signal length or measurement state: %d, %+v, %+v, %+v", len(s.Samples), s.Peak, s.RMS, s.PeakDBFS)
			}
			for i, v := range s.Samples {
				want := float32(0)
				if signal == "constant" || (signal == "impulse" && i == 480) {
					want = 0.5
				}
				if v != want {
					t.Fatalf("sample %d: got %g, want %g", i, v, want)
				}
			}
		})
	}
}

func TestUsesLearnerSamplesAndMetricsWithoutReferenceFallback(t *testing.T) {
	f := unfinishedFunctions()
	f.Sine = func(rate, n int, frequency, amplitude float64) []float32 {
		if rate != SampleRate || n != 960 || frequency != 1000 || amplitude != 0.5 {
			t.Fatal("generator did not receive experiment settings")
		}
		x := make([]float32, n)
		x[0], x[1] = 0.5, -0.25
		return x
	}
	// Intentionally unusual values verify that metrics come from learner calls.
	f.Peak = func(x []float32) float64 { x[0] = 42; return 7 }
	f.RMS = func(x []float32) float64 {
		if x[0] != 0.5 {
			t.Fatal("measurement inputs were not isolated")
		}
		return 8
	}
	f.AmplitudeDB = func(x float64) float64 {
		if x != 7 {
			t.Fatalf("dB conversion received %g, want reported peak 7", x)
		}
		return 9
	}
	e, err := Build(defaultConfig("sine"), f)
	if err != nil {
		t.Fatal(err)
	}
	s := e.Stages[0]
	if s.Samples[0] != 0.5 || *s.Peak.Value != 7 || *s.RMS.Value != 8 || *s.PeakDBFS.Value != 9 {
		t.Fatalf("runner replaced or corrupted the learner's results")
	}
	if e.PlaybackGain != 0.1 {
		t.Fatal("listening gain used the potentially wrong learner metric")
	}
}

func TestStagesRetainOriginalsAndShareListeningGain(t *testing.T) {
	f := unfinishedFunctions()
	// Mutating fixtures verify that the runner still preserves earlier stages.
	f.ApplyGain = func(x []float32, gain float64) []float32 {
		if gain != 16 || x[0] != 16 {
			t.Fatal("incorrect gain stage inputs")
		}
		for i := range x {
			x[i] = 256
		}
		return x
	}
	f.HardClip = func(x []float32) []float32 {
		if x[0] != 256 {
			t.Fatal("clipping did not follow gain")
		}
		for i := range x {
			x[i] = 1
		}
		return x
	}
	gain := 16.0
	c := defaultConfig("constant")
	c.Amplitude, c.Gain, c.Clip = 16, &gain, true
	e, err := Build(c, f)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Stages) != 3 || e.Stages[0].Samples[0] != 16 || e.Stages[1].Samples[0] != 256 || e.Stages[2].Samples[0] != 1 {
		t.Fatal("stage data was overwritten")
	}
	if e.PlaybackGain != 0.9/256 || e.Stages[1].OverRange != 960 || e.Stages[2].OverRange != 0 {
		t.Fatal("incorrect shared attenuation or over-range count")
	}
	for _, s := range e.Stages {
		var wav bytes.Buffer
		if err := WriteWAV(&wav, s.Samples, e.PlaybackGain); err != nil {
			t.Fatal(err)
		}
	}
}

func TestUnfinishedOrInvalidSampleStagesStopExperiment(t *testing.T) {
	for _, tc := range []struct {
		name string
		fn   func(int, int, float64, float64) []float32
		want string
	}{
		{"pending", func(int, int, float64, float64) []float32 {
			panic(exercises.Unimplemented{Lesson: 2, Function: "Sine"})
		}, "lesson 2"},
		{"short", func(int, int, float64, float64) []float32 { return []float32{0} }, "want exactly 960"},
		{"nonfinite", func(_ int, n int, _, _ float64) []float32 {
			x := make([]float32, n)
			x[4] = float32(math.NaN())
			return x
		}, "non-finite sample at index 4"},
		{"panic", func(int, int, float64, float64) []float32 { panic("broken generator") }, "broken generator"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := unfinishedFunctions()
			f.Sine = tc.fn
			_, err := Build(defaultConfig("sine"), f)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want error containing %q", err, tc.want)
			}
		})
	}
}

func TestConfigRejectsInvalidAndUnboundedInputs(t *testing.T) {
	for _, change := range []func(*Config){
		func(c *Config) { c.Signal = "../unknown" },
		func(c *Config) { c.Duration = 0 },
		func(c *Config) { c.Duration = 21 * time.Millisecond },
		func(c *Config) { c.Duration = 11 * time.Second },
		func(c *Config) { c.Amplitude = math.NaN() },
		func(c *Config) { c.Amplitude = -1 },
		func(c *Config) { c.Amplitude = 17 },
		func(c *Config) { c.Frequency = 24000 },
		func(c *Config) { c.Frequency = math.Inf(1) },
		func(c *Config) { c.Signal, c.ImpulseAt = "impulse", 960 },
		func(c *Config) { g := math.Inf(1); c.Gain = &g },
	} {
		c := defaultConfig("silence")
		change(&c)
		if err := c.Validate(); err == nil {
			t.Errorf("accepted invalid config %+v", c)
		}
	}
}

func TestMetricStatesAndSilenceAreJSONSafe(t *testing.T) {
	for _, tc := range []struct {
		fn    func() float64
		db    bool
		state string
	}{
		{func() float64 { return math.Inf(-1) }, true, "ready"},
		{func() float64 { return math.NaN() }, false, "error"},
		{func() float64 { return -1 }, false, "error"},
		{func() float64 { panic("bug") }, false, "error"},
		{func() float64 { panic(exercises.Unimplemented{Lesson: 3}) }, false, "pending"},
	} {
		m := measure(tc.fn, tc.db)
		if m.State != tc.state {
			t.Errorf("got state %s, want %s", m.State, tc.state)
		}
		if _, err := json.Marshal(m); err != nil {
			t.Errorf("metric cannot be saved: %v", err)
		}
	}
}

func TestWAVHeaderSamplesAndNoSilentClipping(t *testing.T) {
	var wav bytes.Buffer
	if err := WriteWAV(&wav, []float32{0, 0.5, -1}, 0.1); err != nil {
		t.Fatal(err)
	}
	b := wav.Bytes()
	if len(b) != 50 || string(b[:4]) != "RIFF" || string(b[8:12]) != "WAVE" || string(b[36:40]) != "data" {
		t.Fatalf("invalid WAV structure")
	}
	if binary.LittleEndian.Uint32(b[4:8]) != 42 || binary.LittleEndian.Uint32(b[24:28]) != 48000 ||
		binary.LittleEndian.Uint16(b[22:24]) != 1 || binary.LittleEndian.Uint16(b[34:36]) != 16 ||
		binary.LittleEndian.Uint32(b[40:44]) != 6 {
		t.Fatal("incorrect WAV size or PCM format")
	}
	for i, want := range []int16{0, 1638, -3277} {
		if got := int16(binary.LittleEndian.Uint16(b[44+i*2:])); got != want {
			t.Errorf("listening sample %d: got %d, want %d", i, got, want)
		}
	}
	if err := WriteWAV(&bytes.Buffer{}, []float32{2}, 1); err == nil {
		t.Fatal("export silently clipped an over-range value")
	}
}

func TestSaveKeepsRunsAndEscapesNotes(t *testing.T) {
	c := defaultConfig("impulse")
	c.Prediction = "</script><script>alert('unexpected')</script>"
	e, err := Build(c, unfinishedFunctions())
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	first, err := Save(root, e)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Save(root, e)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("new run overwrote an existing experiment")
	}
	html, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(html, []byte(c.Prediction)) || !bytes.Contains(html, []byte("Pending")) {
		t.Fatal("notes were not escaped or pending results were hidden")
	}
	var saved Experiment
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(first), "data.json"))
	if err != nil || json.Unmarshal(raw, &saved) != nil || saved.Config.Prediction != c.Prediction || saved.Stages[0].Samples[0] != 0.5 {
		t.Fatal("raw data did not round-trip")
	}
	pointer, err := os.ReadFile(filepath.Join(root, "work", "latest"))
	if err != nil || string(pointer) != second {
		t.Fatal("latest pointer was not updated")
	}
	e.PlaybackGain = 8
	if _, err := Save(root, e); err == nil {
		t.Fatal("invalid export unexpectedly succeeded")
	}
	pointer, err = os.ReadFile(filepath.Join(root, "work", "latest"))
	if err != nil || string(pointer) != second {
		t.Fatal("failed export replaced the last successful report")
	}
	runs, _ := os.ReadDir(filepath.Join(root, "work", "runs"))
	if len(runs) != 2 {
		t.Fatal("failed export left an incomplete run")
	}
}
