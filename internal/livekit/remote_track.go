package livekit

import (
	"time"
)

// RemoteTrack represents a track that is subscribed to by the client.
// It provides methods to read RTP packets and get track information.
type RemoteTrack interface {
	ID() string
	Kind() string
	ReadRTP() ([]byte, error)
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
	return ""
}

func (a *remoteTrackAdapter) ReadRTP() ([]byte, error) {
	return nil, nil
}

func (a *remoteTrackAdapter) SSRC() uint32 {
	return uint32(a.track.SSRC())
}

func (a *remoteTrackAdapter) SetReadDeadline(t time.Time) error {
	return a.track.SetReadDeadline(t)
}
