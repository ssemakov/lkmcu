package livekit

import (
	"time"

	"github.com/pion/webrtc/v4"
)

type Receiver interface {
	ReadRTCP() ([]byte, error)
	SetReadDeadline(t time.Time) error
	Stop() error
}

type recieverAdapter struct {
	receiver *webrtc.RTPReceiver
}

func (a *recieverAdapter) ReadRTCP() ([]byte, error) {
	if a.receiver == nil {
		return nil, nil
	}

	return nil, nil // temp

	// return a.receiver.ReadRTCP()
}

func (a *recieverAdapter) SetReadDeadline(t time.Time) error {
	if a.receiver == nil {
		return nil
	}

	return a.receiver.SetReadDeadline(t)
}

func (a *recieverAdapter) Stop() error {
	if a.receiver == nil {
		return nil
	}

	return a.receiver.Stop()
}
