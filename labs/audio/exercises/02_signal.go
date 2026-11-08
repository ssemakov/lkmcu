package exercises

// Sine allocates samples mono float32 values, beginning at phase zero.
// Sample i represents time i/sampleRate seconds. Keep phase continuous across
// 960-sample boundaries. Inputs: sampleRate > 0, samples >= 0,
// 0 <= frequency < sampleRate/2, and a finite amplitude >= 0.
// Amplitudes above 1 are valid here. Return exactly samples values.
func Sine(sampleRate, samples int, frequency, amplitude float64) []float32 {
	panic(Unimplemented{Lesson: 2, Function: "Sine"})
}
