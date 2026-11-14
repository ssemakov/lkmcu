package exercises

// HardClip returns a new slice whose samples are bounded to [-1, +1].
// Preserve samples that are already in range and leave the input unchanged.
// Inputs are finite. This is a distortion baseline, not the intended limiter.
func HardClip(samples []float32) []float32 {
	clipped := make([]float32, 0, len(samples))

	for _, s := range samples {
		if s > 1 {
			clipped = append(clipped, 1.0)
		} else if s < -1 {
			clipped = append(clipped, -1.0)
		} else {
			clipped = append(clipped, s)
		}
	}

	return clipped
}
