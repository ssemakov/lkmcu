package livekit

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"github.com/ssemakov/lkmcu/internal/media/packet"
)

type fakeWebrtcRemoteTrack struct {
	id   string
	kind webrtc.RTPCodecType
	ssrc webrtcSSRC

	readRTPPacket *rtp.Packet
	readRTPAttrs  interceptor.Attributes
	readRTPErr    error
	readRTPCalled int

	setReadDeadlineArg    time.Time
	setReadDeadlineCalled int
	setReadDeadlineErr    error
}

func (f *fakeWebrtcRemoteTrack) ID() string {
	return f.id
}

func (f *fakeWebrtcRemoteTrack) SetReadDeadline(t time.Time) error {
	f.setReadDeadlineCalled++
	f.setReadDeadlineArg = t
	return f.setReadDeadlineErr
}

func (f *fakeWebrtcRemoteTrack) SSRC() webrtcSSRC {
	return f.ssrc
}

func (f *fakeWebrtcRemoteTrack) ReadRTP() (*rtp.Packet, interceptor.Attributes, error) {
	f.readRTPCalled++
	return f.readRTPPacket, f.readRTPAttrs, f.readRTPErr
}

func (f *fakeWebrtcRemoteTrack) Kind() webrtc.RTPCodecType {
	return f.kind
}

func (f *fakeWebrtcRemoteTrack) Codec() webrtc.RTPCodecParameters {
	return webrtc.RTPCodecParameters{RTPCodecCapability: webrtc.RTPCodecCapability{ClockRate: 90000, MimeType: "audio"}}
}

func TestRemoteTrackAdapter_ID(t *testing.T) {
	fake := &fakeWebrtcRemoteTrack{id: "track-123"}
	a := &remoteTrackAdapter{track: fake}

	got := a.ID()
	if got != "track-123" {
		t.Fatalf("ID mismatch: want %q, got %q", "track-123", got)
	}
}

func TestRemoteTrackAdapter_Kind(t *testing.T) {
	fake := &fakeWebrtcRemoteTrack{kind: webrtc.RTPCodecTypeAudio}
	a := &remoteTrackAdapter{track: fake}

	got := a.Kind()
	if got != "audio" {
		t.Fatalf("Kind mismatch: want %q, got %q", "audio", got)
	}
}

func TestRemoteTrackAdapter_ReadRTP_Delegates(t *testing.T) {
	srcPkt := &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			SequenceNumber: 42,
			Timestamp:      1234,
			SSRC:           5678,
		},
		Payload: []byte{0x1, 0x2, 0x3},
	}
	wantPkt := packet.Packet{
		Seq:     42,
		RtpTs:   1234,
		Payload: []byte{0x1, 0x2, 0x3},
	}
	fake := &fakeWebrtcRemoteTrack{
		readRTPPacket: srcPkt,
		readRTPAttrs:  interceptor.Attributes{"x-test": "ignored"},
	}
	a := &remoteTrackAdapter{track: fake}

	got, err := a.ReadRTP()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if fake.readRTPCalled != 1 {
		t.Fatalf("expected ReadRTP to be called once, got %d", fake.readRTPCalled)
	}
	if got.Seq != wantPkt.Seq || got.RtpTs != wantPkt.RtpTs || !bytes.Equal(got.Payload, wantPkt.Payload) {
		t.Fatalf("packet mismatch: want %#v, got %#v", wantPkt, got)
	}
}

func TestRemoteTrackAdapter_ReadRTP_PropagatesError(t *testing.T) {
	wantErr := errors.New("read failed")
	fake := &fakeWebrtcRemoteTrack{readRTPErr: wantErr}
	a := &remoteTrackAdapter{track: fake}

	got, err := a.ReadRTP()
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected error %v, got %v", wantErr, err)
	}
	if got.Seq != 0 || got.RtpTs != 0 || len(got.Payload) != 0 {
		t.Fatalf("expected zero packet on error, got %#v", got)
	}
}

func TestRemoteTrackAdapter_SSRC(t *testing.T) {
	fake := &fakeWebrtcRemoteTrack{ssrc: webrtcSSRC(777)}
	a := &remoteTrackAdapter{track: fake}

	got := a.SSRC()
	if got != 777 {
		t.Fatalf("SSRC mismatch: want %d, got %d", 777, got)
	}
}

func TestRemoteTrackAdapter_ClockRate(t *testing.T) {
	fake := &fakeWebrtcRemoteTrack{ssrc: webrtcSSRC(777)}
	a := &remoteTrackAdapter{track: fake}

	got := a.ClockRate()
	if got != 90000 {
		t.Fatalf("ClockRate mismatch: want %d, got %d", 90000, got)
	}
}

func TestRemoteTrackAdapter_MimeType(t *testing.T) {
	fake := &fakeWebrtcRemoteTrack{ssrc: webrtcSSRC(777)}
	a := &remoteTrackAdapter{track: fake}

	got := a.MimeType()
	if got != "audio" {
		t.Fatalf("MimeType mismatch: want audio, got %v", got)
	}
}

func TestRemoteTrackAdapter_SetReadDeadline_Delegates(t *testing.T) {
	deadline := time.Date(2026, time.March, 6, 11, 12, 13, 14, time.UTC)
	wantErr := errors.New("deadline failed")
	fake := &fakeWebrtcRemoteTrack{setReadDeadlineErr: wantErr}
	a := &remoteTrackAdapter{track: fake}

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
