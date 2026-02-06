// Package jitter_buffer implements a simple, adaptive RTP jitter buffer
// suitable for audio/video playout. It smooths inter‑arrival jitter by
// admitting, ordering, and releasing packets against a playout cursor on
// a stable clock produced by the clock mapper.
//
// Design notes and expected behavior are described in docs/jitter_buffer.md.
package jitter_buffer

import "time"

// PacketPayload is the raw payload carried by an RTP packet.
type PacketPayload []byte

// Packet represents a single RTP packet with both its original RTP
// metadata and a mapped presentation timestamp on the master clock.
//
// Fields:
//   - RtpTs: RTP timestamp as received on the wire.
//   - PayloadTs: Presentation timestamp on the master clock (NTP‑like),
//     filled by the jitter buffer using the clock mapper during Push.
//   - Seq: RTP sequence number.
//   - Payload: raw payload bytes.
type Packet struct {
	RtpTs     uint32
	PayloadTs time.Time
	Seq       uint16
	Payload   PacketPayload
}

// Less reports whether this Packet should sort before the other Packet.
// Packets are ordered first by PayloadTs, then by Seq to break ties.
// This ordering is used by the jitter buffer's packet heap.
func (p *Packet) Less(other *Packet) bool {
	if p.PayloadTs.Equal(other.PayloadTs) {
		return p.Seq < other.Seq
	}
	return p.PayloadTs.Before(other.PayloadTs)
}
