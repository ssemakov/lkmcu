package processor

import (
	"time"

	"github.com/ssemakov/lkmcu/internal/media/jitter"
	"github.com/ssemakov/lkmcu/internal/media/packet"
)

type FakeBuffer struct {
	packets []packet.Packet
}

func (b *FakeBuffer) Push(_ uint32, pkt packet.Packet) (jitter.Status, error) {
	b.packets = append(b.packets, pkt)
	return jitter.StatusAccepted, nil
}

func (b *FakeBuffer) Pop(_ uint32, _ time.Time) (packet.Packet, bool) {
	if len(b.packets) == 0 {
		return packet.Packet{}, false
	}
	pkt := b.packets[0]
	b.packets = b.packets[1:]
	return pkt, true
}

func (b *FakeBuffer) Occupancy(uint32) time.Duration { return 0 }
func (b *FakeBuffer) Target(uint32) time.Duration    { return 0 }
func (b *FakeBuffer) Config() *jitter.Config {
	return jitter.DefaultConfig()
}
