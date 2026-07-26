package depaketizers

import (
	"github.com/ssemakov/lkmcu/internal/media/packet"
)

type OpusDepaketizer struct {
}

func NewOpusDepaketizer() *OpusDepaketizer {
	return &OpusDepaketizer{}
}

func (d *OpusDepaketizer) Open() error {
	// Initialize Opus depaketizer resources here.
	return nil
}

func (d *OpusDepaketizer) Close() error {
	// Clean up Opus depaketizer resources here.
	return nil
}

func (d *OpusDepaketizer) Depaketize(packet packet.Packet) (EncodedUnit, error) {
	// Implement Opus depaketization logic here.
	// This typically involves parsing the RTP payload and extracting the Opus frames.
	return nil, nil
}
