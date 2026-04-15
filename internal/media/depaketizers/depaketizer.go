package depaketizers

import "github.com/ssemakov/lkmcu/internal/media/packet"

type EncodedUnit interface {
}

type Depaketizer interface {
	Open() error
	Close() error
	Depaketize(packet.Packet) (EncodedUnit, error)
}
