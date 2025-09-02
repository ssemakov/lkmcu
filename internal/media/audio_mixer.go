package media

type AudioMixer interface {
	AddSource(id string) error
	RemoveSource(id string) error
	Mix(dst []float32)
}
