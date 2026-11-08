package exercises

// ApplyGain returns a new slice with every sample multiplied by gain.
// Preserve the input, including its backing storage. Negative gains are valid.
// Do not clamp. Inputs and their products are finite and fit in float32.
func ApplyGain(samples []float32, gain float64) []float32 {
	panic(Unimplemented{Lesson: 4, Function: "ApplyGain"})
}

// AmplitudeDB expresses the magnitude of an amplitude relative to 1.0 in dB.
// A negative amplitude has the same level as its positive magnitude.
// Return negative infinity for zero. Inputs are finite.
// Applied to a measured peak, this is sample-peak dBFS.
func AmplitudeDB(amplitude float64) float64 {
	panic(Unimplemented{Lesson: 4, Function: "AmplitudeDB"})
}
