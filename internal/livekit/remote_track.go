package livekit

import (
	"time"

	"github.com/pion/rtp"
)

// RemoteTrack represents a track that is subscribed to by the client.
// It provides methods to read RTP packets and get track information.
type RemoteTrack interface {
	ID() string
	Kind() string
	ReadRTP() (*rtp.Packet, error)
	SSRC() uint32
	SetReadDeadline(time.Time) error
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

func (a *remoteTrackAdapter) ReadRTP() (*rtp.Packet, error) {
	pkts, _, err := a.track.ReadRTP()
	return pkts, err
}

func (a *remoteTrackAdapter) SSRC() uint32 {
	return uint32(a.track.SSRC())
}

func (a *remoteTrackAdapter) SetReadDeadline(t time.Time) error {
	return a.track.SetReadDeadline(t)
}
