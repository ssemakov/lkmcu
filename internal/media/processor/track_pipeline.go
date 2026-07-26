// Package processor wires track-level ingest and render loops on top of
// clock mapping, jitter buffering, depacketization, and decoding.
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
	"github.com/ssemakov/lkmcu/internal/media"
	"github.com/ssemakov/lkmcu/internal/media/clock"
	"github.com/ssemakov/lkmcu/internal/media/decoders"
	"github.com/ssemakov/lkmcu/internal/media/depaketizers"
	"github.com/ssemakov/lkmcu/internal/media/jitter"
	"github.com/ssemakov/lkmcu/internal/media/packet"
)

// TrackPipeline processes one remote track from RTP/RTCP ingest through
// buffered rendering.
//
// Contract:
//   - Start() launches the RTP and RTCP read loops for the track.
//   - Stop() cancels the loops, releases receiver resources, and waits
//     for the pipeline goroutines to exit.
//   - Render() drains packets whose playout time has arrived and passes
//     them through depacketization and decoding.
type TrackPipeline interface {
	Start()
	Stop()
	Render(now time.Time)
}

// TrackPipelineFactory creates per-track processing pipelines.
type TrackPipelineFactory interface {
	NewTrackPipeline(track livekit.RemoteTrack, receiver livekit.Receiver) (TrackPipeline, error)
}

// TrackPipelineConfig controls per-track RTP and RTCP read deadlines.
type TrackPipelineConfig struct {
	RTPReadDeadline  time.Duration
	RTCPReadDeadline time.Duration
}

// DefaultTrackPipelineFactory builds track pipelines using shared context,
// logging, clock mapping, and jitter buffering components.
type DefaultTrackPipelineFactory struct {
	Logger       logging.Logger
	Ctx          context.Context
	Config       TrackPipelineConfig
	Mapper       *clock.ClockMapper
	Buffer       jitter.Buffer
	CodecFactory media.CodecFactory
}

type trackPipeline struct {
	buffer      jitter.Buffer
	mapper      *clock.ClockMapper
	track       livekit.RemoteTrack
	receiver    livekit.Receiver
	config      TrackPipelineConfig
	depaketizer depaketizers.Depaketizer
	decoder     decoders.Decoder

	ctx    context.Context
	cancel context.CancelFunc
	logger logging.Logger
	wg     sync.WaitGroup
}

// DefaultTrackPipelineConfig returns conservative read deadlines suitable
// for the RTP and RTCP polling loops.
func DefaultTrackPipelineConfig() TrackPipelineConfig {
	return TrackPipelineConfig{
		RTPReadDeadline:  100 * time.Millisecond,
		RTCPReadDeadline: 100 * time.Millisecond,
	}
}

// NewTrackPipeline creates a TrackPipeline for the given remote track and
// receiver. It validates the factory context, applies default config when
// needed, and selects depacketizer/decoder implementations from the track
// MIME type.
func (f *DefaultTrackPipelineFactory) NewTrackPipeline(track livekit.RemoteTrack, receiver livekit.Receiver) (TrackPipeline, error) {
	if f.Ctx == nil {
		return nil, errors.New("context is required in TrackPipelineFactory")
	}

	runCtx, cancel := context.WithCancel(f.Ctx)
	config := f.Config
	if config == (TrackPipelineConfig{}) {
		config = DefaultTrackPipelineConfig()
	}

	if f.CodecFactory == nil {
		f.CodecFactory = &media.DefaultCodecFactory{}
	}

	f.Logger.Infof("Creating new track pipeline for track ID: %s, SSRC: %08x, ClockRate: %d, MimeType: %s",
		track.ID(), track.SSRC(), track.ClockRate(), track.MimeType())

	depaketizer, err := f.CodecFactory.NewDepaketizer(track.MimeType())

	if err != nil {
		cancel()
		f.Logger.Errorf("Failed to create depaketizer for track ID: %s, error: %v", track.ID(), err)
		return nil, err
	}

	decoder, err := f.CodecFactory.NewDecoder(track.MimeType())

	if err != nil {
		cancel()
		f.Logger.Errorf("Failed to create decoder for track ID: %s, error: %v", track.ID(), err)
		return nil, err
	}

	return &trackPipeline{
		buffer:      f.Buffer,
		mapper:      f.Mapper,
		track:       track,
		receiver:    receiver,
		config:      config,
		depaketizer: depaketizer,
		decoder:     decoder,

		ctx:    runCtx,
		cancel: cancel,
		logger: f.Logger,
		wg:     sync.WaitGroup{},
	}, nil
}

// Start launches the RTP and RTCP read loops for the track.
// Timeouts are retried, expected closure signals stop the loop quietly,
// and other errors are logged before terminating the pipeline.
func (p *trackPipeline) Start() {
	p.mapper.SetClockRate(p.track.SSRC(), p.track.ClockRate())
	p.wg.Add(2)

	go p.rtpIngestLoop()
	go p.rtcpIngestLoop()
}

// Stop cancels the pipeline context, unblocks pending reads, stops the
// receiver, and waits for the worker goroutines to exit.
func (p *trackPipeline) Stop() {
	p.cancel()

	now := time.Now()

	if err := p.track.SetReadDeadline(now); err != nil {
		p.logger.Errorf("error setting track read deadline during stop: %v", err)
	}

	if err := p.receiver.SetReadDeadline(now); err != nil {
		p.logger.Errorf("error setting receiver read deadline during stop: %v", err)
	}

	if err := p.receiver.Stop(); err != nil {
		p.logger.Errorf("error stopping receiver: %v", err)
	}

	timer := time.AfterFunc(2*time.Second, func() {
		p.logger.Errorf("Still waiting for track pipeline goroutine to stop (track_id=%s)", p.track.ID())
	})
	p.wg.Wait()
	timer.Stop()
}

// Render drains ready packets for the track SSRC and sends them through
// depacketization and decoding. Packets that fail either step are logged
// and skipped.
func (p *trackPipeline) Render(now time.Time) {
	ssrc := p.track.SSRC()

	for {
		pkt, ok := p.buffer.Pop(ssrc, now)
		if !ok {
			// No more packets to render at this time, exit the loop and wait for the next render call
			return
		}
		p.logger.Tracef("Popped packet from buffer for track ID: %s, SSRC: %d, RTP Time: %d", p.track.ID(), ssrc, pkt.RtpTs)

		depaketized, err := p.depaketizer.Depaketize(pkt)
		if err != nil {
			p.logger.Errorf("error depaketizing packet for track ID: %s, SSRC: %d, RTP Time: %d, error: %v",
				p.track.ID(), ssrc, pkt.RtpTs, err)
			continue // skip this packet and continue with the next one
		}
		if err = p.decoder.Accept(depaketized); err != nil {
			p.logger.Errorf("error decoding packet for track ID: %s, SSRC: %d, RTP Time: %d, error: %v",
				p.track.ID(), ssrc, pkt.RtpTs, err)
			continue // skip this packet and continue with the next one
		}
	}

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

func (p *trackPipeline) rtpIngestLoop() {
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
}

func (p *trackPipeline) rtcpIngestLoop() {
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
}

func (p *trackPipeline) handleRTCP() error {
	err := p.receiver.SetReadDeadline(time.Now().Add(p.config.RTCPReadDeadline))
	if err != nil {
		return err
	}

	reports, err := p.receiver.ReadSenderReports()
	if err != nil {
		return err
	}

	err = p.onSR(reports)
	if err != nil {
		return err
	}

	return nil
}

func (p *trackPipeline) onRTP(pkt packet.Packet) error {
	status, err := p.buffer.Push(p.track.SSRC(), pkt)

	switch status {
	case jitter.StatusMapperDegraded:
		p.logger.Debugf("RTP accepted but clock mapper is degraded, track ID: %s", p.track.ID())
	case jitter.StatusAccepted:
		p.logger.Tracef("RTP accepted, track ID: %s", p.track.ID())
	default:
		p.logger.Debugf("RTP packet was not accepted by the buffer, status: %v, track ID: %s",
			jitter.StatusString(status), p.track.ID())
	}

	if err != nil {
		return err
	}

	return nil
}

func (p *trackPipeline) onSR(reports []packet.SenderReport) error {
	for _, sr := range reports {
		err := p.mapper.UpdateFromSR(p.track.SSRC(), sr.RtpTs, sr.NtpHigh, sr.NtpLow)
		if err != nil {
			p.logger.Errorf("Failed to update clock mapper for track %s with SR: RTP TS=%d, NTP High=%d, NTP Low=%d, error: %v",
				p.track.ID(), sr.RtpTs, sr.NtpHigh, sr.NtpLow, err)
		}
	}

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
