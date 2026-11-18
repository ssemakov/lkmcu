package exercises

import "time"

// PCMFormat describes the Phase 1 mixer boundary, not the decoder API.
type PCMFormat struct {
	SampleRate        int
	Channels          int
	Layout            string
	SampleType        string
	BlockDuration     time.Duration
	SamplesPerChannel int
	AllowOverRange    bool
	AllowNonFinite    bool
}

// MixerFormat returns the PCM contract you chose for this study plan.
// Use "mono" for Layout and "float32" for SampleType. Also write the prose
// contract in notes/06-pcm-contract.md, including where range management belongs.
func MixerFormat() PCMFormat {
	return PCMFormat{
		SampleRate:        48000,
		Channels:          1,
		Layout:            "mono",
		SampleType:        "float32",
		BlockDuration:     20 * time.Millisecond,
		SamplesPerChannel: 960,
		AllowOverRange:    true,
		AllowNonFinite:    false,
	}
}
