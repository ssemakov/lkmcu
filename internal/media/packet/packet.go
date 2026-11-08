package packet

import (
	"github.com/pion/rtp"
)

// PacketPayload is the raw payload carried by an RTP packet.
type PacketPayload []byte

// Packet represents a single RTP packet with both its original RTP
// metadata and a mapped presentation timestamp on the master clock.
//
// Fields:
//   - RtpTs: RTP timestamp as received on the wire.
//     filled by the jitter buffer using the clock mapper during Push.
//   - Seq: RTP sequence number.
//   - Payload: raw payload bytes.
type Packet struct {
	RTPTS   uint32
	Seq     uint16
	Payload PacketPayload
}

// FromRTP translates rtp.Packet to Packet
// Payload must be read-only
func FromRTP(src *rtp.Packet) (Packet, error) {
	rtpPacket := Packet{
		RTPTS:   src.Timestamp,
		Seq:     src.SequenceNumber,
		Payload: src.Payload,
	}

	return rtpPacket, nil
}
