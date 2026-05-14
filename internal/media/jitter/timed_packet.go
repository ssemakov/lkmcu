package jitter

import (
	"time"

	"github.com/ssemakov/lkmcu/internal/media/packet"
)

type timedPacket struct {
	packet.Packet
	PayloadTs time.Time
}

// Less reports whether this Packet should sort before the other Packet.
// Packets are ordered first by PayloadTs, then by Seq to break ties.
// This ordering is used by the jitter buffer's packet heap.
func (tp *timedPacket) Less(other *timedPacket) bool {
	if tp.PayloadTs.Equal(other.PayloadTs) {
		return tp.Seq < other.Seq
	}
	return tp.PayloadTs.Before(other.PayloadTs)
}
