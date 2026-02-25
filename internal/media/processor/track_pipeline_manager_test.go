package processor

import (
	"testing"
	"time"

	"github.com/ssemakov/lkmcu/internal/livekit"
)

type fakeTPFactory struct {
	pipeline TrackPipeline
	err      error
	calls    int
}

func (f *fakeTPFactory) NewTrackPipeline(track livekit.RemoteTrack, receiver livekit.Receiver) (TrackPipeline, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.pipeline, nil
}

type fakeTP struct {
	startCalled int
	stopCalled  int
}

func (p *fakeTP) Start() {
	p.startCalled++
}

func (p *fakeTP) Stop() {
	p.stopCalled++
}

type fakeTrack struct {
	id string
}

func (t fakeTrack) ID() string                      { return t.id }
func (t fakeTrack) Kind() string                    { return "video" }
func (t fakeTrack) ReadRTP() ([]byte, error)        { return nil, nil }
func (t fakeTrack) SSRC() uint32                    { return 1234 }
func (t fakeTrack) SetReadDeadline(time.Time) error { return nil }

type fakeReceiver struct {
}

func (r fakeReceiver) ReadRTCP() ([]byte, error)         { return nil, nil }
func (r fakeReceiver) SetReadDeadline(t time.Time) error { return nil }

func TestTrackPipelineManager_NewTrackPipeline(t *testing.T) {
	pipeline := &fakeTP{}
	factory := &fakeTPFactory{pipeline: pipeline}
	m := NewTrackPipelineManagerWithFactory(factory).(*trackPipelineManager)
	track := fakeTrack{id: "t1"}
	receiver := fakeReceiver{}

	got, err := m.NewTrackPipeline(track, receiver)
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
	if pipeline.startCalled != 1 {
		t.Fatalf("Start() calls = %d, want 1", pipeline.startCalled)
	}
}

func TestTrackPipelineManager_RemoveTrackPipeline_NotFound(t *testing.T) {
	m := NewTrackPipelineManagerWithFactory(&fakeTPFactory{pipeline: &fakeTP{}}).(*trackPipelineManager)

	expected := "track pipeline not found for track ID: missing"
	if err := m.RemoveTrackPipeline("missing"); err == nil || err.Error() != expected {
		t.Fatalf("RemoveTrackPipeline() error = %v, want %q", err, expected)
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
