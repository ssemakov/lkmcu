package exercises_test

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/ssemakov/lkmcu/labs/audio/exercises"
)

// pending turns starter panics into useful failures without skipping a lesson.
func pending(t *testing.T) {
	t.Helper()
	if p := recover(); p != nil {
		if todo, ok := p.(exercises.Unimplemented); ok {
			t.Fatal(todo)
		}
		panic(p)
	}
}

func closeTo(t *testing.T, got, want, tolerance float64) {
	t.Helper()
	if math.IsNaN(got) || math.IsInf(got, 0) || math.Abs(got-want) > tolerance {
		t.Errorf("got %.10g, want %.10g (tolerance %g)", got, want, tolerance)
	}
}

func TestLesson01_SamplesPerChannel(t *testing.T) {
	defer pending(t)
	for _, tc := range []struct {
		rate int
		d    time.Duration
		want int
	}{
		{48000, 20 * time.Millisecond, 960},
		{48000, 10 * time.Millisecond, 480},
		{48000, time.Second, 48000},
		{44100, 20 * time.Millisecond, 882},
		{48000, 0, 0},
		{48000, time.Nanosecond, 0},
		{48000, 1500 * time.Microsecond, 72},
		{48000, 30 * time.Microsecond, 1},
	} {
		if got := exercises.SamplesForDuration(tc.rate, tc.d); got != tc.want {
			t.Errorf("rate=%d duration=%s: got %d samples per channel, want %d", tc.rate, tc.d, got, tc.want)
		}
	}
	// Stereo layout is [L0, R0, L1, R1, ...]. A frame has one sample per channel.
	frames := exercises.SamplesForDuration(48000, 20*time.Millisecond)
	if totalValues := frames * 2; totalValues != 1920 {
		t.Errorf("20 ms stereo: got %d scalar values, want 1920 (960 per channel)", totalValues)
	}
}

func TestLesson02_SineSamples(t *testing.T) {
	defer pending(t)
	got := exercises.Sine(8, 8, 1, 0.5)
	want := []float64{0, 0.3535533906, 0.5, 0.3535533906, 0, -0.3535533906, -0.5, -0.3535533906}
	if len(got) != len(want) {
		t.Fatalf("got %d samples, want %d", len(got), len(want))
	}
	for i := range want {
		closeTo(t, float64(got[i]), want[i], 1e-6)
	}
	if got := exercises.Sine(48000, 0, 1000, 0.5); len(got) != 0 {
		t.Errorf("zero samples requested, got %d", len(got))
	}
	for _, v := range exercises.Sine(48000, 48, 1000, 0) {
		closeTo(t, float64(v), 0, 1e-7)
	}
	for _, v := range exercises.Sine(48000, 48, 0, 0.5) {
		closeTo(t, float64(v), 0, 1e-7)
	}
}

func TestLesson02_PhaseAcrossBlocksAndOverRange(t *testing.T) {
	defer pending(t)
	got := exercises.Sine(48000, 1920, 440, 0.5)
	if len(got) != 1920 {
		t.Fatalf("got %d samples, want 1920", len(got))
	}
	// 440 Hz does not complete an integer number of cycles in 20 ms.
	closeTo(t, float64(got[960]), -0.4755282581, 1e-6)
	closeTo(t, float64(got[961]), -0.4658456138, 1e-6)
	over := exercises.Sine(48000, 48, 1000, 1.5)
	if len(over) != 48 {
		t.Fatalf("got %d samples, want 48", len(over))
	}
	closeTo(t, float64(over[12]), 1.5, 1e-6)
	closeTo(t, float64(over[36]), -1.5, 1e-6)
}

func TestLesson03_Peak(t *testing.T) {
	defer pending(t)
	for _, tc := range []struct {
		x    []float32
		want float64
	}{
		{nil, 0}, {[]float32{0, 0, 0}, 0},
		{[]float32{-0.75, 0.5, 0}, 0.75},
		{[]float32{-1.5, 0.5}, 1.5},
		{[]float32{0.5, 0.5, 0.5}, 0.5},
		{[]float32{0, 1, 0, 0}, 1},
	} {
		closeTo(t, exercises.Peak(tc.x), tc.want, 1e-7)
	}
}

func TestLesson03_RMS(t *testing.T) {
	defer pending(t)
	for _, tc := range []struct {
		x    []float32
		want float64
	}{
		{nil, 0}, {[]float32{0, 0}, 0},
		{[]float32{-0.5, 0.5, -0.5, 0.5}, 0.5},
		{[]float32{0.5, 0.5, 0.5, 0.5}, 0.5},
		{[]float32{1, 0, 0, 0}, 0.5},
		{[]float32{0, 1, 0, -1}, 0.7071067812},
		{[]float32{-2, 2}, 2},
		{[]float32{1e20, -1e20}, float64(float32(1e20))},
	} {
		closeTo(t, exercises.RMS(tc.x), tc.want, math.Max(1e-7, tc.want*1e-7))
	}
}

func TestLesson04_GainAndOwnership(t *testing.T) {
	defer pending(t)
	for _, tc := range []struct {
		gain float64
		want []float32
	}{
		{0, []float32{0, 0, 0}},
		{0.5, []float32{-0.25, 0, 0.125}},
		{3, []float32{-1.5, 0, 0.75}},
		{-1, []float32{0.5, 0, -0.25}},
		{1, []float32{-0.5, 0, 0.25}},
	} {
		x := []float32{-0.5, 0, 0.25}
		y := exercises.ApplyGain(x, tc.gain)
		if !reflect.DeepEqual(y, tc.want) {
			t.Errorf("gain %g: got %v, want %v", tc.gain, y, tc.want)
		}
		if !reflect.DeepEqual(x, []float32{-0.5, 0, 0.25}) {
			t.Fatal("ApplyGain changed its input")
		}
		if len(y) > 0 {
			y[0] = 42
			if x[0] != -0.5 {
				t.Fatal("returned slice shares input storage")
			}
		}
	}
	if got := exercises.ApplyGain(nil, 2); len(got) != 0 {
		t.Errorf("empty input: got %v", got)
	}
}

func TestLesson04_AmplitudeDB(t *testing.T) {
	defer pending(t)
	for _, tc := range []struct{ amplitude, want float64 }{
		{1, 0}, {0.5, -6.0205999133}, {2, 6.0205999133},
		{0.1, -20}, {-0.5, -6.0205999133},
	} {
		closeTo(t, exercises.AmplitudeDB(tc.amplitude), tc.want, 1e-7)
	}
	if got := exercises.AmplitudeDB(0); !math.IsInf(got, -1) {
		t.Errorf("silence: got %g, want negative infinity", got)
	}
}

func TestLesson05_HardClip(t *testing.T) {
	defer pending(t)
	x := []float32{-2, -1, -0.25, 0, 0.75, 1, 1.5}
	original := append([]float32(nil), x...)
	want := []float32{-1, -1, -0.25, 0, 0.75, 1, 1}
	y := exercises.HardClip(x)
	if !reflect.DeepEqual(y, want) {
		t.Errorf("got %v, want %v", y, want)
	}
	if !reflect.DeepEqual(x, original) {
		t.Fatal("HardClip changed its input")
	}
	if len(y) > 0 {
		y[0] = 42
		if x[0] != original[0] {
			t.Fatal("returned slice shares input storage")
		}
	}
	if got := exercises.HardClip(nil); len(got) != 0 {
		t.Errorf("empty input: got %v", got)
	}
}

func TestLesson06_PCMContract(t *testing.T) {
	defer pending(t)
	f := exercises.MixerFormat()
	want := exercises.PCMFormat{
		SampleRate: 48000, Channels: 1, Layout: "mono", SampleType: "float32",
		BlockDuration: 20 * time.Millisecond, SamplesPerChannel: 960,
		AllowOverRange: true, AllowNonFinite: false,
	}
	if f != want {
		t.Errorf("PCM contract:\n got: %+v\nwant: %+v\nAlso write notes/06-pcm-contract.md.", f, want)
	}
}
