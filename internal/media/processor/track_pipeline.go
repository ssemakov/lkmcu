package processor

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/pion/transport/v3/packetio"
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
	Start()
	// Stop the pipeline and release resources.
	Stop()
}

// TrackPipelineFactory defines an interface for creating new track pipelines.
type TrackPipelineFactory interface {
	NewTrackPipeline(track livekit.RemoteTrack, receiver livekit.Receiver) (TrackPipeline, error)
}

type TrackPipelineConfig struct {
	RTPReadDeadline  time.Duration
	RTCPReadDeadline time.Duration
}
type DefaultTrackPipelineFactory struct {
	Logger logging.Logger
	Ctx    context.Context
	Config TrackPipelineConfig
}

type trackPipeline struct {
	buffer   *jitter_buffer.Buffer
	mapper   *clock.ClockMapper
	track    livekit.RemoteTrack
	receiver livekit.Receiver
	config   TrackPipelineConfig
	ctx      context.Context
	cancel   context.CancelFunc
	logger   logging.Logger
	wg       sync.WaitGroup
}

func DefaultTrackPipelineConfig() TrackPipelineConfig {
	return TrackPipelineConfig{
		RTPReadDeadline:  100 * time.Millisecond,
		RTCPReadDeadline: 100 * time.Millisecond,
	}
}

func (f *DefaultTrackPipelineFactory) NewTrackPipeline(track livekit.RemoteTrack, receiver livekit.Receiver) (TrackPipeline, error) {
	if f.Ctx == nil {
		return nil, errors.New("context is required in TrackPipelineFactory")
	}

	runCtx, cancel := context.WithCancel(f.Ctx)
	config := f.Config
	if config == (TrackPipelineConfig{}) {
		config = DefaultTrackPipelineConfig()
	}

	return &trackPipeline{
		buffer:   nil,
		mapper:   nil,
		track:    track,
		receiver: receiver,
		config:   config,
		ctx:      runCtx,
		cancel:   cancel,
		logger:   f.Logger,
		wg:       sync.WaitGroup{},
	}, nil
}

func (p *trackPipeline) Start() {
	p.wg.Add(2)

	go func() {
		defer p.wg.Done()

		for {
			select {
			case <-p.ctx.Done():
				return
			default:
				err := p.handleRTP()
				switch {
				case p.isTimeout(err):
					// expected when no packets are available, just return and wait for the next read
					continue
				case errors.Is(err, io.EOF),
					errors.Is(err, io.ErrClosedPipe),
					errors.Is(err, context.Canceled):
					// expected when the track or receiver is closed, just return and stop the pipeline
					return
				default:
					if err != nil {
						p.logger.Errorf("error handling RTP: %v", err)
						return // if there's an error other than timeout or expected closure, stop the pipeline
					}
				}
			}
		}
	}()

	go func() {
		defer p.wg.Done()

		for {
			select {
			case <-p.ctx.Done():
				return
			default:
				err := p.handleRTCP()
				switch {
				case p.isTimeout(err):
					// expected when no packets are available, just return and wait for the next read
					continue
				case errors.Is(err, io.EOF),
					errors.Is(err, io.ErrClosedPipe),
					errors.Is(err, context.Canceled):
					// expected when the track or receiver is closed, just return and stop the pipeline
					return
				default:
					if err != nil {
						p.logger.Errorf("error handling RTCP: %v", err)
						return // if there's an error other than timeout or expected closure, stop the pipeline
					}
				}
			}
		}
	}()
}

func (p *trackPipeline) Stop() {
	p.cancel()
	err := p.track.SetReadDeadline(time.Now())

	if err != nil {
		p.logger.Errorf("error setting track read deadline during stop: %v", err)
	}
	err = p.receiver.SetReadDeadline(time.Now())

	if err != nil {
		p.logger.Errorf("error setting receiver read deadline during stop: %v", err)
	}

	p.receiver.Stop()

	timer := time.AfterFunc(2*time.Second, func() {
		p.logger.Errorf("Still waiting for track pipeline goroutine to stop (track_id=%s)", p.track.ID())
	})
	p.wg.Wait()
	timer.Stop()
}

func (p *trackPipeline) handleRTP() error {
	err := p.track.SetReadDeadline(time.Now().Add(p.config.RTPReadDeadline))

	if err != nil {
		return err
	}

	pkt, err := p.track.ReadRTP()

	if err != nil {
		return err
	}

	// log and return other
	err = p.onRTP(pkt)
	if err != nil {
		return err
	}

	return nil
}

func (p *trackPipeline) handleRTCP() error {
	err := p.receiver.SetReadDeadline(time.Now().Add(p.config.RTCPReadDeadline))
	if err != nil {
		return err
	}

	pkt, err := p.receiver.ReadRTCP()
	if err != nil {
		return err
	}

	err = p.onRTCP(pkt)
	if err != nil {
		return err
	}

	return nil
}

func (p *trackPipeline) onRTP(packet Packet) error {
	// push packet into the jitter buffer for processing.
	return nil
}

func (p *trackPipeline) onRTCP(packet Packet) error {
	// push to updateFromRTCP or other RTCP handling.
	return nil
}

func (p *trackPipeline) isTimeout(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, packetio.ErrTimeout) {
		return true
	}

	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}
