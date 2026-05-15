package jitter

import (
	"time"

	"github.com/ssemakov/lkmcu/internal/media/packet"
)

type timedPacket struct {
	packet.Packet
	PayloadTS time.Time
}

// Less reports whether this Packet should sort before the other Packet.
// Packets are ordered first by PayloadTS, then by Seq to break ties.
// This ordering is used by the jitter buffer's packet heap.
func (tp *timedPacket) Less(other *timedPacket) bool {
	if tp.PayloadTS.Equal(other.PayloadTS) {
		return tp.Seq < other.Seq
	}
	return tp.PayloadTS.Before(other.PayloadTS)
}
