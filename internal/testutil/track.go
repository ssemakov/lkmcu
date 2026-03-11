package testutil

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/ssemakov/lkmcu/internal/livekit"
)

type TestTrack int

const (
	// Timeout variants.
	TimeoutRTPTrack TestTrack = iota

	// Terminal read-error variants.
	EOFRTPTrack
	ClosedPipeRTPTrack
	CanceledRTPTrack
	UnexpectedErrRTPTrack
)

type TestReceiver int

const (
	// Terminal read-error variants.
	EOFReceiver TestReceiver = iota
	TimeoutReceiver
	ClosedPipeReceiver
	CanceledReceiver
	UnexpectedErrReceiver
)

type ReadCallCounter interface {
	ReadCalls() int32
}

type CounterTrack interface {
	livekit.RemoteTrack
	ReadCallCounter
}

type CounterReceiver interface {
	livekit.Receiver
	ReadCallCounter
}

// Track: timeout variant.
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

// Track: terminal read-error variant.
type errorRTPTrack struct {
	id         string
	readErr    error
	readCalled int32
}

func (t *errorRTPTrack) ID() string                      { return t.id }
func (t *errorRTPTrack) Kind() string                    { return "video" }
func (t *errorRTPTrack) SSRC() uint32                    { return 1234 }
func (t *errorRTPTrack) SetReadDeadline(time.Time) error { return nil }
func (t *errorRTPTrack) ReadRTP() (*rtp.Packet, error) {
	atomic.AddInt32(&t.readCalled, 1)
	return nil, t.readErr
}

func (t *errorRTPTrack) ReadCalls() int32 {
	return atomic.LoadInt32(&t.readCalled)
}

// Track: stop-path variant (blocking read until explicit release).
type StopTrack struct {
	id                    string
	readStarted           chan struct{}
	readReturned          chan struct{}
	releaseCh             chan struct{}
	releaseOnce           sync.Once
	readStartedOnce       sync.Once
	readReturnedOnce      sync.Once
	stopDeadlineCalls     int32
	unblockOnStopDeadline bool
	readCalled            int32
}

func newStopTrack(id string, unblockOnStopDeadline bool) *StopTrack {
	return &StopTrack{
		id:                    id,
		readStarted:           make(chan struct{}),
		readReturned:          make(chan struct{}),
		releaseCh:             make(chan struct{}),
		unblockOnStopDeadline: unblockOnStopDeadline,
	}
}

func (t *StopTrack) ID() string   { return t.id }
func (t *StopTrack) Kind() string { return "video" }
func (t *StopTrack) SSRC() uint32 { return 1234 }

func (t *StopTrack) SetReadDeadline(deadline time.Time) error {
	if isStopDeadline(deadline) {
		atomic.AddInt32(&t.stopDeadlineCalls, 1)
		if t.unblockOnStopDeadline {
			t.Release()
		}
	}

	return nil
}

func (t *StopTrack) ReadRTP() (*rtp.Packet, error) {
	atomic.AddInt32(&t.readCalled, 1)
	t.readStartedOnce.Do(func() { close(t.readStarted) })
	<-t.releaseCh
	t.readReturnedOnce.Do(func() { close(t.readReturned) })
	return nil, context.Canceled
}

func (t *StopTrack) ReadCalls() int32 {
	return atomic.LoadInt32(&t.readCalled)
}

func (t *StopTrack) StopDeadlineCalls() int32 {
	return atomic.LoadInt32(&t.stopDeadlineCalls)
}

func (t *StopTrack) ReadStarted() <-chan struct{} {
	return t.readStarted
}

func (t *StopTrack) ReadReturned() <-chan struct{} {
	return t.readReturned
}

func (t *StopTrack) Release() {
	t.releaseOnce.Do(func() { close(t.releaseCh) })
}

// Receiver: terminal io.EOF variant.
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

// Receiver: timeout variant.
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

// Receiver: terminal read-error variant.
type errorReceiver struct {
	readErr    error
	readCalled int32
}

func (r *errorReceiver) ReadRTCP() ([]rtcp.Packet, error) {
	atomic.AddInt32(&r.readCalled, 1)
	return nil, r.readErr
}

func (r *errorReceiver) SetReadDeadline(time.Time) error { return nil }
func (r *errorReceiver) Stop() error                     { return nil }
func (r *errorReceiver) ReadCalls() int32 {
	return atomic.LoadInt32(&r.readCalled)
}

// Receiver: stop-path variant (blocking read until explicit release).
type StopReceiver struct {
	readStarted           chan struct{}
	readReturned          chan struct{}
	stopCalled            chan struct{}
	releaseCh             chan struct{}
	releaseOnce           sync.Once
	stopCalledOnce        sync.Once
	readStartedOnce       sync.Once
	readReturnedOnce      sync.Once
	stopCalls             int32
	stopDeadlineCalls     int32
	unblockOnStopDeadline bool
	readCalled            int32
}

func newStopReceiver(unblockOnStopDeadline bool) *StopReceiver {
	return &StopReceiver{
		readStarted:           make(chan struct{}),
		readReturned:          make(chan struct{}),
		stopCalled:            make(chan struct{}),
		releaseCh:             make(chan struct{}),
		unblockOnStopDeadline: unblockOnStopDeadline,
	}
}

func (r *StopReceiver) ReadRTCP() ([]rtcp.Packet, error) {
	atomic.AddInt32(&r.readCalled, 1)
	r.readStartedOnce.Do(func() { close(r.readStarted) })
	<-r.releaseCh
	r.readReturnedOnce.Do(func() { close(r.readReturned) })
	return nil, context.Canceled
}

func (r *StopReceiver) SetReadDeadline(deadline time.Time) error {
	if isStopDeadline(deadline) {
		atomic.AddInt32(&r.stopDeadlineCalls, 1)
		if r.unblockOnStopDeadline {
			r.Release()
		}
	}

	return nil
}

func (r *StopReceiver) Stop() error {
	atomic.AddInt32(&r.stopCalls, 1)
	r.stopCalledOnce.Do(func() { close(r.stopCalled) })
	return nil
}

func (r *StopReceiver) ReadCalls() int32 {
	return atomic.LoadInt32(&r.readCalled)
}

func (r *StopReceiver) StopCalls() int32 {
	return atomic.LoadInt32(&r.stopCalls)
}

func (r *StopReceiver) StopDeadlineCalls() int32 {
	return atomic.LoadInt32(&r.stopDeadlineCalls)
}

func (r *StopReceiver) ReadStarted() <-chan struct{} {
	return r.readStarted
}

func (r *StopReceiver) ReadReturned() <-chan struct{} {
	return r.readReturned
}

func (r *StopReceiver) StopCalled() <-chan struct{} {
	return r.stopCalled
}

func (r *StopReceiver) Release() {
	r.releaseOnce.Do(func() { close(r.releaseCh) })
}

type timeoutNetError struct{}

func (timeoutNetError) Error() string   { return "timeout" }
func (timeoutNetError) Timeout() bool   { return true }
func (timeoutNetError) Temporary() bool { return true }

var errUnexpectedRead = errors.New("unexpected read failure")

func isStopDeadline(deadline time.Time) bool {
	return deadline.Before(time.Now().Add(time.Second))
}

func MakeRTPTrackAndReceiver(
	trackType TestTrack,
	receiverType TestReceiver,
) (CounterTrack, CounterReceiver, error) {
	var (
		track CounterTrack
	)

	switch trackType {
	case TimeoutRTPTrack:
		t := &timeoutRTPTrack{id: "t1"}
		track = t
	case EOFRTPTrack:
		t := &errorRTPTrack{id: "t1", readErr: io.EOF}
		track = t
	case ClosedPipeRTPTrack:
		t := &errorRTPTrack{id: "t1", readErr: io.ErrClosedPipe}
		track = t
	case CanceledRTPTrack:
		t := &errorRTPTrack{id: "t1", readErr: context.Canceled}
		track = t
	case UnexpectedErrRTPTrack:
		t := &errorRTPTrack{id: "t1", readErr: errUnexpectedRead}
		track = t
	default:
		return nil, nil, fmt.Errorf("unsupported track type: %d", trackType)
	}

	var (
		receiver CounterReceiver
	)

	switch receiverType {
	case EOFReceiver:
		r := &eofReceiver{}
		receiver = r
	case TimeoutReceiver:
		r := &timeoutReceiver{}
		receiver = r
	case ClosedPipeReceiver:
		r := &errorReceiver{readErr: io.ErrClosedPipe}
		receiver = r
	case CanceledReceiver:
		r := &errorReceiver{readErr: context.Canceled}
		receiver = r
	case UnexpectedErrReceiver:
		r := &errorReceiver{readErr: errUnexpectedRead}
		receiver = r
	default:
		return nil, nil, fmt.Errorf("unsupported receiver type: %d", receiverType)
	}

	return track, receiver, nil
}

func MakeStopTrackAndReceiver(
	unblockOnStopDeadline bool,
) (*StopTrack, *StopReceiver) {
	track := newStopTrack("t1", unblockOnStopDeadline)
	receiver := newStopReceiver(unblockOnStopDeadline)

	return track, receiver
}
