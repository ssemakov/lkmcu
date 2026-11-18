package media

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

// MixerFormat returns the PCM contract for this mixer.
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

type AudioMixer interface {
	AddSource(id string) error
	RemoveSource(id string) error
	Mix(dst []float32)
}
