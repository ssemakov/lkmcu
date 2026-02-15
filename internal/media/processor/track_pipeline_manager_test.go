package processor

import (
	"errors"
	"testing"

	lk "github.com/ssemakov/lkmcu/internal/livekit"
)

type fakeTPFactory struct {
	pipeline TrackPipeline
	err      error
	calls    int
}

func (f *fakeTPFactory) NewTrackPipeline(track lk.RemoteTrack) (TrackPipeline, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.pipeline, nil
}

type fakeTP struct {
	stopErr    error
	stopCalled int
}

func (p *fakeTP) Start() error               { return nil }
func (p *fakeTP) OnRTP(packet Packet) error  { return nil }
func (p *fakeTP) OnRTCP(packet Packet) error { return nil }
func (p *fakeTP) Stop() error {
	p.stopCalled++
	return p.stopErr
}

type fakeTrack struct {
	id string
}

func (t fakeTrack) ID() string               { return t.id }
func (t fakeTrack) Kind() string             { return "video" }
func (t fakeTrack) ReadRTP() ([]byte, error) { return nil, nil }
func (t fakeTrack) SSRC() uint32             { return 1234 }

func TestTrackPipelineManager_NewTrackPipeline(t *testing.T) {
	factory := &fakeTPFactory{pipeline: &fakeTP{}}
	m := NewTrackPipelineManagerWithFactory(factory).(*trackPipelineManager)
	track := fakeTrack{id: "t1"}

	got, err := m.NewTrackPipeline(track)
	if err != nil {
		t.Fatalf("NewTrackPipeline() error = %v", err)
	}
	if got == nil {
		t.Fatal("NewTrackPipeline() returned nil pipeline")
	}
	if factory.calls != 1 {
		t.Fatalf("factory calls = %d, want 1", factory.calls)
	}
	if _, ok := m.pipelines["t1"]; !ok {
		t.Fatal("pipeline for track t1 was not stored")
	}
}

func TestTrackPipelineManager_RemoveTrackPipeline_NotFound(t *testing.T) {
	m := NewTrackPipelineManagerWithFactory(&fakeTPFactory{pipeline: &fakeTP{}}).(*trackPipelineManager)

	if err := m.RemoveTrackPipeline("missing"); err != nil {
		t.Fatalf("RemoveTrackPipeline() error = %v, want nil", err)
	}
}

func TestTrackPipelineManager_RemoveTrackPipeline(t *testing.T) {
	p := &fakeTP{}
	m := NewTrackPipelineManagerWithFactory(&fakeTPFactory{pipeline: p}).(*trackPipelineManager)
	m.pipelines["t1"] = p

	if err := m.RemoveTrackPipeline("t1"); err != nil {
		t.Fatalf("RemoveTrackPipeline() error = %v", err)
	}
	if p.stopCalled != 1 {
		t.Fatalf("Stop() calls = %d, want 1", p.stopCalled)
	}
	if _, ok := m.pipelines["t1"]; ok {
		t.Fatal("pipeline t1 should be deleted after successful Stop")
	}
}

func TestTrackPipelineManager_RemoveTrackPipeline_StopError(t *testing.T) {
	wantErr := errors.New("stop failed")
	p := &fakeTP{stopErr: wantErr}
	m := NewTrackPipelineManagerWithFactory(&fakeTPFactory{pipeline: p}).(*trackPipelineManager)
	m.pipelines["t1"] = p

	err := m.RemoveTrackPipeline("t1")
	if !errors.Is(err, wantErr) {
		t.Fatalf("RemoveTrackPipeline() error = %v, want %v", err, wantErr)
	}
	if p.stopCalled != 1 {
		t.Fatalf("Stop() calls = %d, want 1", p.stopCalled)
	}
	if _, ok := m.pipelines["t1"]; !ok {
		t.Fatal("pipeline t1 should remain when Stop returns error")
	}
}
