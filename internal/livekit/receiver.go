package livekit

import (
	"time"

	"github.com/pion/webrtc/v4"
)

type Receiver interface {
	ReadRTCP() ([]byte, error)
	SetReadDeadline(t time.Time) error
}

type recieverAdapter struct {
	receiver *webrtc.RTPReceiver
}

func (a *recieverAdapter) ReadRTCP() ([]byte, error) {
	return nil, nil
}

func (a *recieverAdapter) SetReadDeadline(t time.Time) error {
	return a.receiver.SetReadDeadline(t)
}
