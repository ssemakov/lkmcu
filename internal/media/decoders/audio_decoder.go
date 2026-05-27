package decoders

import (
	"time"

	"github.com/ssemakov/lkmcu/internal/media/depaketizers"
)

// EncodedAudioUnit contains output of audio depacketization, that servies as input to audio decode.
type EncodedAudioUnit struct {
	Codec      CodecID
	Payload    []byte
	Duration   time.Duration
	Channels   int
	SampleRate int
	Meta       DecodeMeta
}

type PCMFormat struct {
	SampleRate int
	Channels   int
	Layout     string
	SampleType string
}

// PCMBlock is a fixed internal mixer/render quantum.
type PCMBlock struct {
	Format       PCMFormat
	Samples      []float32
	SamplesPerCh int
	StartAt      time.Time
	Duration     time.Duration
	Concealed    bool // PLC / silence / synthesized
}

// AudioDecoder is a basic Audio decoder interface, to be implemented by specific codec decoders (e.g., OpusDecoder)
type AudioDecoder interface {
	Open() error
	Close() error
	Reset() error
	Accept(depaketizers.EncodedUnit) error
	ReadPCM(samplePerCh int) (PCMBlock, error)
}
