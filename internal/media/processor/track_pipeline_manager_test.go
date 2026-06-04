package processor

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ssemakov/lkmcu/internal/livekit"
	"github.com/ssemakov/lkmcu/internal/logging"
	"github.com/ssemakov/lkmcu/internal/media/packet"
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
	startCalled  int32
	stopCalled   int32
	renderCalled int32
}

func (p *fakeTP) Start() {
	atomic.AddInt32(&p.startCalled, 1)
}

func (p *fakeTP) Stop() {
	atomic.AddInt32(&p.stopCalled, 1)
}

func (p *fakeTP) Render(time.Time) {
	atomic.AddInt32(&p.renderCalled, 1)
}

func (p *fakeTP) RenderCalls() int32 {
	return atomic.LoadInt32(&p.renderCalled)
}

type fakeTrack struct {
	id       string
	ssrc     uint32
	mimeType string
}

func (t fakeTrack) ID() string                      { return t.id }
func (t fakeTrack) Kind() string                    { return "audio" }
func (t fakeTrack) ReadRTP() (packet.Packet, error) { return packet.Packet{}, nil }
func (t fakeTrack) SSRC() uint32                    { return t.ssrc }
func (t fakeTrack) ClockRate() uint32               { return 48000 }
func (t fakeTrack) MimeType() string                { return "audio/opus" }
func (t fakeTrack) SetReadDeadline(time.Time) error { return nil }

type fakeReceiver struct{}

func (r fakeReceiver) ReadSenderReports() ([]packet.SenderReport, error) {
	return nil, nil
}
func (r fakeReceiver) SetReadDeadline(t time.Time) error { return nil }
func (r fakeReceiver) Stop() error                       { return nil }

func newTestTrackPipelineManager(factory TrackPipelineFactory, ctx context.Context) *trackPipelineManager {
	if ctx == nil {
		ctx = context.Background()
	}

	return NewTrackPipelineManagerWithFactory(
		factory,
		newTestMapper(),
		noopBuffer{},
		logging.NewNullLogger("test", "null"),
		ctx,
	).(*trackPipelineManager)
}

func TestTrackPipelineManager_NewTrackPipeline(t *testing.T) {
	pipeline := &fakeTP{}
	factory := &fakeTPFactory{pipeline: pipeline}
	m := newTestTrackPipelineManager(factory, nil)
	track := fakeTrack{id: "t1"}
	receiver := fakeReceiver{}

	got, err := m.NewTrackPipeline("t1", track, receiver)
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

func TestTrackPipelineManager_NewTrackPipeline_audioLoop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pipeline := &fakeTP{}
	m := newTestTrackPipelineManager(&fakeTPFactory{pipeline: pipeline}, ctx)
	track := fakeTrack{id: "t1"}
	receiver := fakeReceiver{}

	_, err := m.NewTrackPipeline("t1", track, receiver)
	if err != nil {
		t.Fatalf("NewTrackPipeline() error = %v", err)
	}

	done := make(chan struct{})
	go func() {
		m.audioLoop()
		close(done)
	}()

	waitForReadCall(t, "Render", pipeline.RenderCalls, 1)

	cancel()
	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected audioLoop to stop after context cancellation")
	}
}

func TestTrackPipelineManager_RemoveTrackPipeline_MultipleCalls(t *testing.T) {
	m := newTestTrackPipelineManager(&fakeTPFactory{pipeline: &fakeTP{}}, nil)
	p := &fakeTP{}
	m.pipelines["t1"] = p

	if err := m.RemoveTrackPipeline("t1"); err != nil {
		t.Fatalf("RemoveTrackPipeline() error = %v", err)
	}

	if _, ok := m.pipelines["t1"]; ok {
		t.Fatal("pipeline t1 should be deleted after successful Stop")
	}

	expected := "track pipeline not found"
	if err := m.RemoveTrackPipeline("missing"); err == nil || err.Error() != expected {
		t.Fatalf("RemoveTrackPipeline() error = %v, want %q", err, expected)
	}

	if p.stopCalled != 1 {
		t.Fatalf("Stop() calls = %d, want 1", p.stopCalled)
	}
}
