package decoders

import "github.com/ssemakov/lkmcu/internal/media"

type OpusDecoder struct {
}

func NewOpusDecoder() *OpusDecoder {
	return &OpusDecoder{}
}

func (d *OpusDecoder) Open() error {
	// Initialize Opus decoder resources here.
	return nil
}

func (d *OpusDecoder) Close() error {
	// Clean up Opus decoder resources here.
	return nil
}

func (d *OpusDecoder) Reset() error {
	// Reset the Opus decoder state here.
	return nil
}

func (d *OpusDecoder) Accept(unit media.EncodedUnit) error {
	// Process the encoded audio unit and decode it into PCM format.
	return nil
}

func (d *OpusDecoder) ReadPCM(samplePerCh int) (media.PCMBlock, error) {
	// Read decoded PCM audio data and return it as a PCMBlock.
	return media.PCMBlock{}, nil
}
