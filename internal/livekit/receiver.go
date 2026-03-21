package livekit

import (
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
	"github.com/ssemakov/lkmcu/internal/media/packet"
)

type Receiver interface {
	// ReadRTCP() ([]rtcp.Packet, error)
	ReadSenderReports() ([]packet.SenderReport, error)
	SetReadDeadline(t time.Time) error
	Stop() error
}

// Internal interface to abstract away the webrtc.RTPReceiver and allow for testing without a full LiveKit connection.
type webRtcRtpReceiver interface {
	ReadRTCP() ([]rtcp.Packet, interceptor.Attributes, error)
	SetReadDeadline(t time.Time) error
	Stop() error
}

// compile-time assertions
var (
	_ Receiver          = (*receiverAdapter)(nil)
	_ webRtcRtpReceiver = (*webrtc.RTPReceiver)(nil)
)

type receiverAdapter struct {
	receiver webRtcRtpReceiver
}

func (a *receiverAdapter) ReadSenderReports() ([]packet.SenderReport, error) {
	if a.receiver == nil {
		return nil, nil
	}

	pkts, _, err := a.receiver.ReadRTCP()
	if err != nil {
		return nil, err
	}

	reports := make([]packet.SenderReport, 0)
	for _, pkt := range pkts {
		sr, ok := pkt.(*rtcp.SenderReport)
		if !ok {
			continue
		}
		reports = append(reports, packet.FromRtcpSR(*sr))
	}

	return reports, nil
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
