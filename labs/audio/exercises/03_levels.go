package exercises

import "math"

// Peak returns the largest absolute sample value, as float64. Empty input is 0.
// Inputs contain finite float32 values. Do not clamp values above 1.
func Peak(samples []float32) float64 {
	peak := 0.0
	for _, s := range samples {
		val := math.Abs(float64(s))
		if val > peak {
			peak = val
		}
	}
	return peak
}

// RMS returns the root mean square of finite samples, as float64.
// Empty input is 0. Use float64 for intermediate arithmetic.
func RMS(samples []float32) float64 {
	if len(samples) == 0 {
		return 0
	}

	sum := 0.0
	for _, s := range samples {
		v := float64(s)
		sum += v * v
	}

	return math.Sqrt(sum / float64(len(samples)))
}
