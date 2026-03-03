package livekit

import (
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
)

type Receiver interface {
	ReadRTCP() ([]rtcp.Packet, error)
	SetReadDeadline(t time.Time) error
	Stop() error
}

type receiverAdapter struct {
	receiver *webrtc.RTPReceiver
}

func (a *receiverAdapter) ReadRTCP() ([]rtcp.Packet, error) {
	if a.receiver == nil {
		return nil, nil
	}

	rtcp, _, err := a.receiver.ReadRTCP()
	if err != nil {
		return nil, err
	}

	return rtcp, nil
}

func (a *receiverAdapter) SetReadDeadline(t time.Time) error {
	if a.receiver == nil {
		return nil
	}

	return a.receiver.SetReadDeadline(t)
}

func (a *receiverAdapter) Stop() error {
	if a.receiver == nil {
		return nil
	}

	return a.receiver.Stop()
}
