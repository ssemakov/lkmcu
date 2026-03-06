package livekit

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
)

type fakeWebRtcRtpReceiver struct {
	readRTCPPackets []rtcp.Packet
	readRTCPAttrs   interceptor.Attributes
	readRTCPErr     error
	readRTCPCalled  int

	setReadDeadlineArg    time.Time
	setReadDeadlineCalled int
	setReadDeadlineErr    error

	stopCalled int
	stopErr    error
}

func (f *fakeWebRtcRtpReceiver) ReadRTCP() ([]rtcp.Packet, interceptor.Attributes, error) {
	f.readRTCPCalled++
	return f.readRTCPPackets, f.readRTCPAttrs, f.readRTCPErr
}

func (f *fakeWebRtcRtpReceiver) SetReadDeadline(t time.Time) error {
	f.setReadDeadlineCalled++
	f.setReadDeadlineArg = t
	return f.setReadDeadlineErr
}

func (f *fakeWebRtcRtpReceiver) Stop() error {
	f.stopCalled++
	return f.stopErr
}

func TestReceiverAdapter_ReadRTCP_NilReceiver(t *testing.T) {
	a := &receiverAdapter{receiver: nil}

	got, err := a.ReadRTCP()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil packets, got %#v", got)
	}
}

func TestReceiverAdapter_ReadRTCP_Delegates(t *testing.T) {
	want := []rtcp.Packet{
		&rtcp.PictureLossIndication{MediaSSRC: 1234},
		&rtcp.TransportLayerNack{MediaSSRC: 4321},
	}
	fake := &fakeWebRtcRtpReceiver{
		readRTCPPackets: want,
		readRTCPAttrs:   interceptor.Attributes{"x-test": "ignored"},
	}
	a := &receiverAdapter{receiver: fake}

	got, err := a.ReadRTCP()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if fake.readRTCPCalled != 1 {
		t.Fatalf("expected ReadRTCP to be called once, got %d", fake.readRTCPCalled)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("packets mismatch: want %#v, got %#v", want, got)
	}
}

func TestReceiverAdapter_ReadRTCP_PropagatesError(t *testing.T) {
	wantErr := errors.New("read failed")
	fake := &fakeWebRtcRtpReceiver{readRTCPErr: wantErr}
	a := &receiverAdapter{receiver: fake}

	got, err := a.ReadRTCP()
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected error %v, got %v", wantErr, err)
	}
	if got != nil {
		t.Fatalf("expected nil packets on error, got %#v", got)
	}
}

func TestReceiverAdapter_SetReadDeadline_NilReceiver(t *testing.T) {
	a := &receiverAdapter{receiver: nil}

	if err := a.SetReadDeadline(time.Now()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestReceiverAdapter_SetReadDeadline_Delegates(t *testing.T) {
	deadline := time.Date(2026, time.March, 5, 10, 11, 12, 13, time.UTC)
	wantErr := errors.New("deadline failed")
	fake := &fakeWebRtcRtpReceiver{setReadDeadlineErr: wantErr}
	a := &receiverAdapter{receiver: fake}

	err := a.SetReadDeadline(deadline)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected error %v, got %v", wantErr, err)
	}
	if fake.setReadDeadlineCalled != 1 {
		t.Fatalf("expected SetReadDeadline to be called once, got %d", fake.setReadDeadlineCalled)
	}
	if !fake.setReadDeadlineArg.Equal(deadline) {
		t.Fatalf("deadline mismatch: want %v, got %v", deadline, fake.setReadDeadlineArg)
	}
}

func TestReceiverAdapter_Stop_NilReceiver(t *testing.T) {
	a := &receiverAdapter{receiver: nil}

	if err := a.Stop(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestReceiverAdapter_Stop_Delegates(t *testing.T) {
	wantErr := errors.New("stop failed")
	fake := &fakeWebRtcRtpReceiver{stopErr: wantErr}
	a := &receiverAdapter{receiver: fake}

	err := a.Stop()
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected error %v, got %v", wantErr, err)
	}
	if fake.stopCalled != 1 {
		t.Fatalf("expected Stop to be called once, got %d", fake.stopCalled)
	}
}
