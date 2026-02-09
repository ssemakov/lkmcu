package media

import (
	"github.com/ssemakov/lkmcu/internal/logging"
	"github.com/ssemakov/lkmcu/internal/media/clock"
	"github.com/ssemakov/lkmcu/internal/media/jitter_buffer"
)

// Temp interface until I figure out the RTP packet interface.
type Packet interface {
}

type Pipeline interface {
	// Start the pipeline.
	Start() error
	// Stop the pipeline and release resources.
	Stop() error
	// OnRTP is called when a new RTP packet is received for processing.
	OnRTP(packet Packet) error
}

type processor struct {
	buffer *jitter_buffer.Buffer
	mapper *clock.ClockMapper
	logger logging.Logger
}

func NewPipeline(logger logging.Logger) Pipeline {
	return &processor{
		buffer: nil,
		mapper: nil,
		logger: logger,
	}
}

func (p *processor) Start() error {
	return nil
}

func (p *processor) Stop() error {
	return nil
}

func (p *processor) OnRTP(packet Packet) error {
	return nil
}
