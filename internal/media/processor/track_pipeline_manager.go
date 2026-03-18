package processor

import (
	"context"
	"errors"

	"github.com/ssemakov/lkmcu/internal/livekit"
	"github.com/ssemakov/lkmcu/internal/logging"
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
	buffer    jitter_buffer.Buffer
	mapper    *clock.ClockMapper
}

// NewTrackPipelineManager creates a new instance of trackPipelineManager with the provided factory.
// factory - A TrackPipelineFactory used to create new track pipelines.
func NewTrackPipelineManager(l logging.Logger, ctx context.Context) TrackPipelineManager {
	clock_mapper := clock.NewClockMapper(metrics.NewMetrics(), l, clock.DefaultClockMapperConfig())
	// ToDo: pass onOverflowHandler
	buffer := jitter_buffer.NewBuffer(clock_mapper, l, jitter_buffer.DefaultConfig(), nil)

	return &trackPipelineManager{
		factory: &DefaultTrackPipelineFactory{
			Logger: l,
			Ctx:    ctx,
			Config: DefaultTrackPipelineConfig(),
			Mapper: clock_mapper,
			Buffer: buffer,
		},
		pipelines: make(map[string]TrackPipeline),
	}
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

	m.pipelines[track.ID()] = pipeline
	m.pipelines[track.ID()].Start()
	return pipeline, nil
}

func (m *trackPipelineManager) RemoveTrackPipeline(trackID string) error {
	pipeline, ok := m.pipelines[trackID]
	if !ok {
		return errors.New("track pipeline not found for track ID: " + trackID)
	}

	pipeline.Stop()

	delete(m.pipelines, trackID)
	return nil
}
