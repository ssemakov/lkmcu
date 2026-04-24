package processor

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ssemakov/lkmcu/internal/livekit"
	"github.com/ssemakov/lkmcu/internal/logging"
	"github.com/ssemakov/lkmcu/internal/media"
	"github.com/ssemakov/lkmcu/internal/media/clock"
	"github.com/ssemakov/lkmcu/internal/media/jitter_buffer"
	"github.com/ssemakov/lkmcu/internal/metrics"
)

// TrackPipelineManager manages the lifecycle of track pipelines for subscribed tracks.
type TrackPipelineManager interface {
	// NewTrackPipeline creates a new track pipeline for the given remote track and starts processing it.
	NewTrackPipeline(track livekit.RemoteTrack, receiver livekit.Receiver) (TrackPipeline, error)
	// RemoveTrackPipeline stops and removes the track pipeline associated with the given track ID.
	RemoveTrackPipeline(trackID string) error
}

type trackPipelineManager struct {
	factory   TrackPipelineFactory
	pipelines map[string]TrackPipeline
	buffer    *jitter_buffer.Buffer
	mapper    *clock.ClockMapper

	logger logging.Logger
	ctx    context.Context
	mu     sync.Mutex
}

var ErrPipekNotFound = errors.New("track pipeline not found")

// NewTrackPipelineManager creates a new instance of trackPipelineManager with the provided factory.
// factory - A TrackPipelineFactory used to create new track pipelines.
func NewTrackPipelineManager(l logging.Logger, ctx context.Context) TrackPipelineManager {
	clock_mapper := clock.NewClockMapper(metrics.NewMetrics(), l, clock.DefaultClockMapperConfig())
	// ToDo: pass onOverflowHandler
	buffer := jitter_buffer.NewBuffer(clock_mapper, l, jitter_buffer.DefaultConfig(), nil)

	tpm := trackPipelineManager{
		factory: &DefaultTrackPipelineFactory{
			Logger:       l,
			Ctx:          ctx,
			Config:       DefaultTrackPipelineConfig(),
			Mapper:       clock_mapper,
			Buffer:       buffer,
			CodecFactory: &media.DefaultCodecFactory{},
		},
		pipelines: make(map[string]TrackPipeline),
		buffer:    &buffer,
		mapper:    clock_mapper,
		logger:    l,
		ctx:       ctx,
	}

	// start once and let it idle when no ingress
	// TODO: for now all pipelines are audio, we'll introduce video later and can split the loop then
	go tpm.audioLoop()

	return &tpm
}

func NewTrackPipelineManagerWithFactory(factory TrackPipelineFactory) TrackPipelineManager {
	return &trackPipelineManager{
		factory:   factory,
		pipelines: make(map[string]TrackPipeline),
	}
}

func (m *trackPipelineManager) NewTrackPipeline(track livekit.RemoteTrack, receiver livekit.Receiver) (TrackPipeline, error) {
	pipeline, err := m.factory.NewTrackPipeline(track, receiver)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.pipelines[track.ID()] = pipeline
	m.pipelines[track.ID()].Start()
	return pipeline, nil
}

func (m *trackPipelineManager) RemoveTrackPipeline(trackID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pipeline, ok := m.pipelines[trackID]
	if !ok {
		return ErrPipekNotFound
	}

	pipeline.Stop()

	delete(m.pipelines, trackID)
	return nil
}

func (m *trackPipelineManager) audioLoop() {
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-tick.C:
			now := time.Now() // this represent the master clock time for all pipelines
			activeAudioPipelines := m.snapshotPipelines()

			for _, pipeline := range activeAudioPipelines {
				pipeline.Render(now)
			}
		}
	}
}

func (m *trackPipelineManager) snapshotPipelines() []TrackPipeline {
	m.mu.Lock()
	defer m.mu.Unlock()

	pipelines := make([]TrackPipeline, 0, len(m.pipelines))
	for _, pipeline := range m.pipelines {
		pipelines = append(pipelines, pipeline)
	}
	return pipelines
}
