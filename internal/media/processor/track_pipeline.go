package processor

import (
	"github.com/ssemakov/lkmcu/internal/livekit"
	"github.com/ssemakov/lkmcu/internal/logging"
	"github.com/ssemakov/lkmcu/internal/media/clock"
	"github.com/ssemakov/lkmcu/internal/media/jitter_buffer"
)

// Temp interface until I figure out the RTP packet interface.
type Packet interface {
}

// TrackPipeline defines interface for processing single track.
// There's per track pipeline instance.
// The registry of pipelines is maintained by the TracksRuntime
type TrackPipeline interface {
	// Start the pipeline.
	Start() error
	// Stop the pipeline and release resources.
	Stop() error
	// OnRTP is called when a new RTP packet is received for processing.
	OnRTP(packet Packet) error
	OnRTCP(packet Packet) error

	// OnTick is called periodically to process buffered packets and output frames.
	// OnTick(packet Packet) error
}

// TrackPipelineFactory defines an interface for creating new track pipelines.
type TrackPipelineFactory interface {
	NewTrackPipeline(track livekit.RemoteTrack) (TrackPipeline, error)
}

type DefaultTrackPipelineFactory struct {
	Logger logging.Logger
}

func (f *DefaultTrackPipelineFactory) NewTrackPipeline(track livekit.RemoteTrack) (TrackPipeline, error) {
	return &trackPipeline{
		buffer: nil,
		mapper: nil,
		logger: f.Logger,
	}, nil
}

type trackPipeline struct {
	buffer *jitter_buffer.Buffer
	mapper *clock.ClockMapper
	logger logging.Logger
}

func (p *trackPipeline) Start() error {
	// starts go routine to process RTP packets from the buffer and output frames.
	return nil
}

func (p *trackPipeline) Stop() error {
	// stops the processing go routine and releases resources.
	return nil
}

func (p *trackPipeline) OnRTP(packet Packet) error {
	// push packet into the jitter buffer for processing.
	return nil
}

func (p *trackPipeline) OnRTCP(packet Packet) error {
	// push to updateFromRTCP or other RTCP handling.
	return nil
}
