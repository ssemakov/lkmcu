package testutil

import (
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/ssemakov/lkmcu/internal/livekit"
)

type TestTrack int

const (
	TimeoutRTPTrack TestTrack = iota
)

type TestReceiver int

const (
	EOFReceiver TestReceiver = iota
	TimeoutReceiver
)

type ReadCallCounter interface {
	ReadCalls() int32
}

type timeoutRTPTrack struct {
	id         string
	readCalled int32
}

func (t *timeoutRTPTrack) ID() string                      { return t.id }
func (t *timeoutRTPTrack) Kind() string                    { return "video" }
func (t *timeoutRTPTrack) SSRC() uint32                    { return 1234 }
func (t *timeoutRTPTrack) SetReadDeadline(time.Time) error { return nil }
func (t *timeoutRTPTrack) ReadRTP() (*rtp.Packet, error) {
	atomic.AddInt32(&t.readCalled, 1)
	return nil, timeoutNetError{}
}

func (t *timeoutRTPTrack) ReadCalls() int32 {
	return atomic.LoadInt32(&t.readCalled)
}

type eofReceiver struct {
	readCalled int32
}

func (r *eofReceiver) ReadRTCP() ([]rtcp.Packet, error) {
	atomic.AddInt32(&r.readCalled, 1)
	return nil, io.EOF
}

func (r *eofReceiver) SetReadDeadline(time.Time) error { return nil }
func (r *eofReceiver) Stop() error                     { return nil }
func (r *eofReceiver) ReadCalls() int32 {
	return atomic.LoadInt32(&r.readCalled)
}

type timeoutReceiver struct {
	readCalled int32
}

func (r *timeoutReceiver) ReadRTCP() ([]rtcp.Packet, error) {
	atomic.AddInt32(&r.readCalled, 1)
	return nil, timeoutNetError{}
}

func (r *timeoutReceiver) SetReadDeadline(time.Time) error { return nil }
func (r *timeoutReceiver) Stop() error                     { return nil }
func (r *timeoutReceiver) ReadCalls() int32 {
	return atomic.LoadInt32(&r.readCalled)
}

type timeoutNetError struct{}

func (timeoutNetError) Error() string   { return "timeout" }
func (timeoutNetError) Timeout() bool   { return true }
func (timeoutNetError) Temporary() bool { return true }

func MakeRTPTrackAndReceiver(
	trackType TestTrack,
	receiverType TestReceiver,
) (livekit.RemoteTrack, livekit.Receiver, ReadCallCounter, ReadCallCounter, error) {
	var (
		track        livekit.RemoteTrack
		trackCounter ReadCallCounter
	)

	switch trackType {
	case TimeoutRTPTrack:
		t := &timeoutRTPTrack{id: "t1"}
		track = t
		trackCounter = t
	default:
		return nil, nil, nil, nil, fmt.Errorf("unsupported track type: %d", trackType)
	}

	var (
		receiver        livekit.Receiver
		receiverCounter ReadCallCounter
	)

	switch receiverType {
	case EOFReceiver:
		r := &eofReceiver{}
		receiver = r
		receiverCounter = r
	case TimeoutReceiver:
		r := &timeoutReceiver{}
		receiver = r
		receiverCounter = r
	default:
		return nil, nil, nil, nil, fmt.Errorf("unsupported receiver type: %d", receiverType)
	}

	return track, receiver, trackCounter, receiverCounter, nil
}
