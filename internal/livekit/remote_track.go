package livekit

import (
	"time"

	"github.com/ssemakov/lkmcu/internal/media/packet"
)

// RemoteTrack represents a track that is subscribed to by the client.
// It provides methods to read RTP packets and get track information.
type RemoteTrack interface {
	ID() string
	Kind() string
	ReadRTP() (packet.Packet, error)
	SSRC() uint32
	SetReadDeadline(time.Time) error
	ClockRate() uint32
}

// remoteTrackAdapter adapts a webrtc.TrackRemote to the RemoteTrack interface.
type remoteTrackAdapter struct {
	track webrtcRemoteTrack
}

func (a *remoteTrackAdapter) ID() string {
	return a.track.ID()
}

func (a *remoteTrackAdapter) Kind() string {
	return a.track.Kind().String()
}

func (a *remoteTrackAdapter) ReadRTP() (packet.Packet, error) {
	pkt, _, err := a.track.ReadRTP()

	if err != nil {
		return packet.Packet{}, err
	}

	return packet.FromRTP(pkt)
}

func (a *remoteTrackAdapter) SSRC() uint32 {
	return uint32(a.track.SSRC())
}

func (a *remoteTrackAdapter) ClockRate() uint32 {
	codec := a.track.Codec()
	return codec.ClockRate

}

func (a *remoteTrackAdapter) SetReadDeadline(t time.Time) error {
	return a.track.SetReadDeadline(t)
}
