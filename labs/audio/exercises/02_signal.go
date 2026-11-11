package exercises

import "math"

// Sine allocates samples mono float32 values, beginning at phase zero.
// Sample i represents time i/sampleRate seconds. Keep phase continuous across
// 960-sample boundaries. Inputs: sampleRate > 0, samples >= 0,
// 0 <= frequency < sampleRate/2, and a finite amplitude >= 0.
// Amplitudes above 1 are valid here. Return exactly samples values.
func Sine(sampleRate, samples int, frequency, amplitude float64) []float32 {
	samps := make([]float32, 0, samples)

	for i := range samples {
		t := float64(i) / float64(sampleRate)
		angle := math.Pi * frequency * t * 2
		val := amplitude * math.Sin(angle)

		samps = append(samps, float32(val))
	}
	return samps
}
