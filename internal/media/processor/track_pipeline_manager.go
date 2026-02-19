package processor

import (
	"context"
	"errors"

	"github.com/ssemakov/lkmcu/internal/livekit"
	"github.com/ssemakov/lkmcu/internal/logging"
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
}

// NewTrackPipelineManager creates a new instance of trackPipelineManager with the provided factory.
// factory - A TrackPipelineFactory used to create new track pipelines.
func NewTrackPipelineManager(l logging.Logger, ctx context.Context) TrackPipelineManager {
	return &trackPipelineManager{
		factory:   &DefaultTrackPipelineFactory{Logger: l, Ctx: ctx, Config: DefaultTrackPipelineConfig()},
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
