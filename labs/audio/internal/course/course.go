// Package course holds the six fixed lessons. It is deliberately a small course,
// not a configurable learning platform.
package course

import (
	"embed"
	"fmt"
)

//go:embed content/*.md
var content embed.FS

type Lesson struct {
	ID    int
	Title string
	File  string
	Slug  string
	Hints []string
}

var Lessons = []Lesson{
	{1, "Samples and time", "01_time.go", "01-time", []string{
		"A sample rate counts values per second PER CHANNEL. Convert the duration to seconds before relating the two units.",
		"Multiply the rate by the fractional number of seconds. A time.Duration's underlying unit is nanoseconds; converting it to int seconds first loses subsecond time.",
		"Pseudocode: express the duration as a fraction of one second, multiply by rate, discard the fractional sample. For stereo, the number of scalar values is twice the samples per channel.",
	}},
	{2, "What samples represent", "02_signal.go", "02-signal", []string{
		"The sine function expects an angle in radians. Frequency describes cycles per second. Work out the time of sample i.",
		"At time t, a phase-zero sine is amplitude × sin(2π × frequency × t). Use the absolute sample index, not i modulo 960.",
		"Allocate the requested length. For each i: t = float64(i)/float64(sampleRate); calculate the angle and sine in float64, then store float32. Do not clamp.",
	}},
	{3, "Measuring a block", "03_levels.go", "03-levels", []string{
		"Peak concerns the largest magnitude, including negative excursions. RMS measures energy without cancellation of opposite signs.",
		"RMS means square each sample, average those squares, then take the square root. A constant ±0.5 has RMS 0.5; a unit impulse in four samples has RMS 0.5.",
		"Handle an empty slice before division. Peak: track the maximum absolute value. RMS: convert each sample to float64 before squaring, sum, divide by count, then take the square root.",
	}},
	{4, "Gain and dBFS", "04_gain.go", "04-gain", []string{
		"Gain changes each signed value. A negative gain flips polarity. dB compares magnitudes on a logarithmic scale.",
		"For amplitudes, dB = 20 × log10(abs(amplitude)/1.0). Zero is negative infinity. Multiplying amplitude by 2 adds about 6.02 dB.",
		"Allocate independent output for ApplyGain, including gain=1. Calculate products in float64 and convert for storage. For AmplitudeDB, handle zero, then use the logarithm of the magnitude. Do not clamp either function.",
	}},
	{5, "Beyond full scale", "05_clip.go", "05-clip", []string{
		"An internal value of 1.5 is representable in float32. Hard clipping is an explicit operation that changes the signal's shape.",
		"Values above +1 become +1; values below -1 become -1. Leave every in-range value alone. Make independent output storage.",
		"For each input value, choose the lower boundary, upper boundary, or the original value according to its range. The input must remain unchanged so the report can compare stages.",
	}},
	{6, "Your PCM contract", "06_contract.go", "06-contract", []string{
		"Describe what crosses the mixer boundary: rate, channels, memory layout, type, and the interval represented by one block.",
		"Use the study plan's 48 kHz, mono float32, 20 ms format. Internal floating-point headroom is different from the final output ceiling.",
		"The contract uses 960 samples per channel, Layout=mono, SampleType=float32, permits finite over-range values, and excludes NaN/Inf. Write why output range management comes after summation in your paragraph.",
	}},
}

func Get(id int) (Lesson, error) {
	if id < 1 || id > len(Lessons) {
		return Lesson{}, fmt.Errorf("lesson must be between 1 and %d", len(Lessons))
	}
	return Lessons[id-1], nil
}

func (l Lesson) Text() (string, error) {
	b, err := content.ReadFile("content/" + l.Slug + ".md")
	return string(b), err
}

func (l Lesson) TestPattern() string { return fmt.Sprintf("^TestLesson%02d_", l.ID) }
