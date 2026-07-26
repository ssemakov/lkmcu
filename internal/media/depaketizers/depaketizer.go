// Package depaketizers defines codec-specific boundaries for converting RTP
// packets into encoded media units ready for decoders.
package depaketizers

import "github.com/ssemakov/lkmcu/internal/media/packet"

type EncodedUnit any

type Depaketizer interface {
	Open() error
	Close() error
	Depaketize(packet.Packet) (EncodedUnit, error)
}
