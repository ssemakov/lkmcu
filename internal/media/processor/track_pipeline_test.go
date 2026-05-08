package processor

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/ssemakov/lkmcu/internal/logging"
	"github.com/ssemakov/lkmcu/internal/media/clock"
	"github.com/ssemakov/lkmcu/internal/media/decoders"
	"github.com/ssemakov/lkmcu/internal/media/depaketizers"
	"github.com/ssemakov/lkmcu/internal/media/jitter_buffer"
	"github.com/ssemakov/lkmcu/internal/media/packet"
	"github.com/ssemakov/lkmcu/internal/metrics"
)

type noopBuffer struct{}

func (noopBuffer) Push(uint32, packet.Packet) (jitter_buffer.Status, error) {
	return jitter_buffer.StatusAccepted, nil
}

func (noopBuffer) Pop(uint32, time.Time) (packet.Packet, bool) { return packet.Packet{}, false }
func (noopBuffer) Occupancy(uint32) time.Duration              { return 0 }
func (noopBuffer) Target(uint32) time.Duration                 { return 0 }
func (noopBuffer) Config() *jitter_buffer.Config               { return jitter_buffer.DefaultConfig() }

type recordingBuffer struct {
	noopBuffer

	ssrc   uint32
	pkt    packet.Packet
	status jitter_buffer.Status
	err    error
	calls  int
}

func (b *recordingBuffer) Push(ssrc uint32, pkt packet.Packet) (jitter_buffer.Status, error) {
	b.calls++
	b.ssrc = ssrc
	b.pkt = pkt
	return b.status, b.err
}

type fakeDepaketizer struct {
	depaketizeCount int
	depaketizeFn    func(packet.Packet) (depaketizers.EncodedUnit, error)
}

func (de *fakeDepaketizer) Open() error  { return nil }
func (de *fakeDepaketizer) Close() error { return nil }
func (de *fakeDepaketizer) Depaketize(pkt packet.Packet) (depaketizers.EncodedUnit, error) {
	de.depaketizeCount++

	if de.depaketizeFn != nil {
		return de.depaketizeFn(pkt)
	}

	return nil, nil
}

type fakeDecoder struct {
	acceptCount int
	acceptFn    func(depaketizers.EncodedUnit) error
}

func (de *fakeDecoder) Open() error  { return nil }
func (de *fakeDecoder) Close() error { return nil }
func (de *fakeDecoder) Reset() error { return nil }
func (de *fakeDecoder) Accept(eu depaketizers.EncodedUnit) error {
	de.acceptCount++

	if de.acceptFn != nil {
		return de.acceptFn(eu)
	}

	return nil
}

func newTestMapper() *clock.ClockMapper {
	logger := logging.NewNullLogger("test", "null")
	return clock.NewClockMapper(metrics.NewMetrics(), logger, clock.DefaultClockMapperConfig())
}

type fakeCodecFactory struct{}

func (fakeCodecFactory) CreateDecoder(mimeType string) (decoders.Decoder, error) {
	return &fakeDecoder{}, nil
}

func (fakeCodecFactory) CreateDepaketizer(mimeType string) (depaketizers.Depaketizer, error) {
	return &fakeDepaketizer{}, nil
}

func TestDefaultTrackPipelineFactory_NewTrackPipeline_CtxNilError(t *testing.T) {
	factory := &DefaultTrackPipelineFactory{
		Ctx: nil,
	}

	pipeline, err := factory.NewTrackPipeline(fakeTrack{id: "t1"}, fakeReceiver{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "context is required in TrackPipelineFactory" {
		t.Fatalf("unexpected error: %v", err)
	}
	if pipeline != nil {
		t.Fatalf("expected nil pipeline, got %#v", pipeline)
	}
}

func TestDefaultTrackPipelineFactory_NewTrackPipeline_DefaultConfigFallback(t *testing.T) {
	factory := &DefaultTrackPipelineFactory{
		Ctx:    context.Background(),
		Logger: logging.NewNullLogger("test", "null"),
		// zero-value config should fall back to defaults
		Config: TrackPipelineConfig{},
	}

	pipeline, err := factory.NewTrackPipeline(fakeTrack{id: "t1"}, fakeReceiver{})
	if err != nil {
		t.Fatalf("NewTrackPipeline() error = %v", err)
	}

	got, ok := pipeline.(*trackPipeline)
	if !ok {
		t.Fatalf("expected *trackPipeline, got %T", pipeline)
	}

	wantCfg := DefaultTrackPipelineConfig()
	if got.config != wantCfg {
		t.Fatalf("config mismatch: want %+v, got %+v", wantCfg, got.config)
	}
	if got.ctx == nil {
		t.Fatal("expected non-nil run context")
	}
	if got.cancel == nil {
		t.Fatal("expected non-nil cancel func")
	}
}

func TestDefaultTrackPipelineFactory_NewTrackPipeline_CodecFactoryFallback(t *testing.T) {
	factory := &DefaultTrackPipelineFactory{
		Ctx:    context.Background(),
		Logger: logging.NewNullLogger("test", "null"),
		// nil CodecFactory should fall back to DefaultCodecFactory
		CodecFactory: nil,
	}

	pipeline, err := factory.NewTrackPipeline(fakeTrack{id: "t1", mimeType: "audio/opus"}, fakeReceiver{})
	if err != nil {
		t.Fatalf("NewTrackPipeline() error = %v", err)
	}

	got, ok := pipeline.(*trackPipeline)
	if !ok {
		t.Fatalf("expected *trackPipeline, got %T", pipeline)
	}

	if _, ok := got.depaketizer.(*depaketizers.OpusDepaketizer); !ok {
		t.Fatalf("expected DefaultDepaketizer, got %T", got.depaketizer)
	}
	if _, ok := got.decoder.(*decoders.OpusDecoder); !ok {
		t.Fatalf("expected DefaultDecoder, got %T", got.decoder)
	}
}

func TestTrackPipeline_StartLoops(t *testing.T) {
	t.Run("rtp loop continues on timeout", func(t *testing.T) {
		ctx := context.Background()
		track, receiver, err := MakeRTPTrackAndReceiver(
			TimeoutRTPTrack,
			EOFReceiver,
		)
		if err != nil {
			t.Fatalf("MakeTimeoutRTPTrackPipeline() error = %v", err)
		}

		factory := &DefaultTrackPipelineFactory{
			Ctx:    ctx,
			Config: DefaultTrackPipelineConfig(),
			Logger: logging.NewNullLogger("test", "null"),
			Mapper: newTestMapper(),
			Buffer: noopBuffer{},
		}
		p, err := factory.NewTrackPipeline(track, receiver)
		if err != nil {
			t.Fatalf("NewTrackPipeline() error = %v", err)
		}

		p.Start()
		defer p.Stop()

		waitForReadCall(t, "ReadRTP", track.ReadCalls, 2)
	})

	t.Run("rtcp loop continues on timeout", func(t *testing.T) {
		ctx := context.Background()
		track, receiver, err := MakeRTPTrackAndReceiver(
			TimeoutRTPTrack,
			TimeoutReceiver,
		)
		if err != nil {
			t.Fatalf("MakeTimeoutRTPTrackPipeline() error = %v", err)
		}

		factory := &DefaultTrackPipelineFactory{
			Ctx:    ctx,
			Config: DefaultTrackPipelineConfig(),
			Logger: logging.NewNullLogger("test", "null"),
			Mapper: newTestMapper(),
			Buffer: noopBuffer{},
		}
		p, err := factory.NewTrackPipeline(track, receiver)
		if err != nil {
			t.Fatalf("NewTrackPipeline() error = %v", err)
		}

		p.Start()
		defer p.Stop()

		waitForReadCall(t, "ReadRTCP", receiver.ReadCalls, 2)
	})

	t.Run("loops stop on expected closure errors", func(t *testing.T) {
		testCases := []struct {
			name         string
			trackType    TestTrack
			receiverType TestReceiver
		}{
			{
				name:         "io.EOF",
				trackType:    EOFRTPTrack,
				receiverType: EOFReceiver,
			},
			{
				name:         "io.ErrClosedPipe",
				trackType:    ClosedPipeRTPTrack,
				receiverType: ClosedPipeReceiver,
			},
			{
				name:         "context.Canceled",
				trackType:    CanceledRTPTrack,
				receiverType: CanceledReceiver,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				track, receiver, err := MakeRTPTrackAndReceiver(
					tc.trackType,
					tc.receiverType,
				)
				if err != nil {
					t.Fatalf("MakeRTPTrackAndReceiver() error = %v", err)
				}

				factory := &DefaultTrackPipelineFactory{
					Ctx:    context.Background(),
					Config: DefaultTrackPipelineConfig(),
					Logger: logging.NewNullLogger("test", "null"),
					Mapper: newTestMapper(),
					Buffer: noopBuffer{},
				}

				p, err := factory.NewTrackPipeline(track, receiver)
				if err != nil {
					t.Fatalf("NewTrackPipeline() error = %v", err)
				}

				p.Start()
				defer p.Stop()

				waitForReadCall(t, "ReadRTP", track.ReadCalls, 1)
				waitForReadCall(t, "ReadRTCP", receiver.ReadCalls, 1)

				// No more reads should happen after this
				time.Sleep(40 * time.Millisecond)
				var expected int32 = 1 // loop should stop immediately after the first terminal error

				if got := track.ReadCalls(); got != expected {
					t.Fatalf("expected RTP loop to stop after %s, calls changed from %d to %d", tc.name, expected, got)
				}
				if got := receiver.ReadCalls(); got != 1 {
					t.Fatalf("expected RTCP loop to stop after %s, calls changed from %d to %d", tc.name, expected, got)
				}
			})
		}
	})

	t.Run("loops stop on unexpected errors", func(t *testing.T) {
		track, receiver, err := MakeRTPTrackAndReceiver(
			UnexpectedErrRTPTrack,
			UnexpectedErrReceiver,
		)
		if err != nil {
			t.Fatalf("MakeRTPTrackAndReceiver() error = %v", err)
		}

		factory := &DefaultTrackPipelineFactory{
			Ctx:    context.Background(),
			Config: DefaultTrackPipelineConfig(),
			Logger: logging.NewNullLogger("test", "null"),
			Mapper: newTestMapper(),
			Buffer: noopBuffer{},
		}

		p, err := factory.NewTrackPipeline(track, receiver)
		if err != nil {
			t.Fatalf("NewTrackPipeline() error = %v", err)
		}

		p.Start()
		defer p.Stop()

		waitForReadCall(t, "ReadRTP", track.ReadCalls, 1)
		waitForReadCall(t, "ReadRTCP", receiver.ReadCalls, 1)

		time.Sleep(40 * time.Millisecond)
		var expected int32 = 1 // loop should stop immediately after the first terminal error

		if got := track.ReadCalls(); got != expected {
			t.Fatalf("expected RTP loop to stop after unexpected error, calls changed from %d to %d", expected, got)
		}
		if got := receiver.ReadCalls(); got != expected {
			t.Fatalf("expected RTCP loop to stop after unexpected error, calls changed from %d to %d", expected, got)
		}
	})
}

func TestTrackPipeline_Stop(t *testing.T) {
	t.Run("cancels context and unblocks readers", func(t *testing.T) {
		track, receiver := MakeStopTrackAndReceiver(
			true,
		)

		factory := &DefaultTrackPipelineFactory{
			Ctx: context.Background(),
			Config: TrackPipelineConfig{
				RTPReadDeadline:  10 * time.Second,
				RTCPReadDeadline: 10 * time.Second,
			},
			Logger: logging.NewNullLogger("test", "null"),
			Mapper: newTestMapper(),
			Buffer: noopBuffer{},
		}

		p, err := factory.NewTrackPipeline(track, receiver)
		if err != nil {
			t.Fatalf("NewTrackPipeline() error = %v", err)
		}

		tp, ok := p.(*trackPipeline)
		if !ok {
			t.Fatalf("expected *trackPipeline, got %T", p)
		}

		p.Start()
		waitForSignal(t, "ReadRTP started", track.ReadStarted(), 250*time.Millisecond)
		waitForSignal(t, "ReadRTCP started", receiver.ReadStarted(), 250*time.Millisecond)

		stopDone := make(chan struct{})
		go func() {
			p.Stop()
			close(stopDone)
		}()

		select {
		case <-stopDone:
		case <-time.After(250 * time.Millisecond):
			track.Release()
			receiver.Release()
			t.Fatal("Stop() did not return after cancel/deadline unblock")
		}

		if !errors.Is(tp.ctx.Err(), context.Canceled) {
			t.Fatalf("expected canceled context after Stop, got %v", tp.ctx.Err())
		}
		if got := track.StopDeadlineCalls(); got == 0 {
			t.Fatalf("expected track SetReadDeadline(now) to be called, got %d", got)
		}
		if got := receiver.StopDeadlineCalls(); got == 0 {
			t.Fatalf("expected receiver SetReadDeadline(now) to be called, got %d", got)
		}

		waitForSignal(t, "ReadRTP returned", track.ReadReturned(), 150*time.Millisecond)
		waitForSignal(t, "ReadRTCP returned", receiver.ReadReturned(), 150*time.Millisecond)
	})

	t.Run("stops receiver and waits for goroutines", func(t *testing.T) {
		track, receiver := MakeStopTrackAndReceiver(
			false,
		)

		factory := &DefaultTrackPipelineFactory{
			Ctx: context.Background(),
			Config: TrackPipelineConfig{
				RTPReadDeadline:  10 * time.Second,
				RTCPReadDeadline: 10 * time.Second,
			},
			Logger: logging.NewNullLogger("test", "null"),
			Mapper: newTestMapper(),
			Buffer: noopBuffer{},
		}

		p, err := factory.NewTrackPipeline(track, receiver)
		if err != nil {
			t.Fatalf("NewTrackPipeline() error = %v", err)
		}

		p.Start()
		waitForSignal(t, "ReadRTP started", track.ReadStarted(), 250*time.Millisecond)
		waitForSignal(t, "ReadRTCP started", receiver.ReadStarted(), 250*time.Millisecond)

		stopDone := make(chan struct{})
		go func() {
			p.Stop()
			close(stopDone)
		}()

		waitForSignal(t, "receiver Stop called", receiver.StopCalled(), 250*time.Millisecond)

		select {
		case <-stopDone:
			t.Fatal("Stop() returned before reader goroutines were released")
		case <-time.After(40 * time.Millisecond):
			// expected: Stop should still be waiting for goroutines
		}

		track.Release()
		receiver.Release()

		select {
		case <-stopDone:
		case <-time.After(250 * time.Millisecond):
			t.Fatal("Stop() did not return after readers were released")
		}

		if got := receiver.StopCalls(); got != 1 {
			t.Fatalf("expected receiver.Stop() to be called once, got %d", got)
		}
	})

	t.Run("tolerates receiver Stop failure and still waits for goroutines", func(t *testing.T) {
		track, receiver := MakeStopTrackAndReceiver(false)

		failingReceiver := StopErrorReceiver{
			StopReceiver: receiver,
			Err:          errors.New("stop failed"),
		}

		factory := &DefaultTrackPipelineFactory{
			Ctx: context.Background(),
			Config: TrackPipelineConfig{
				RTPReadDeadline:  10 * time.Second,
				RTCPReadDeadline: 10 * time.Second,
			},
			Logger: logging.NewNullLogger("test", "null"),
			Mapper: newTestMapper(),
			Buffer: noopBuffer{},
		}

		p, err := factory.NewTrackPipeline(track, &failingReceiver)
		if err != nil {
			t.Fatalf("NewTrackPipeline() error = %v", err)
		}

		p.Start()

		waitForSignal(t, "ReadRTP started", track.ReadStarted(), 250*time.Millisecond)
		waitForSignal(t, "ReadRTCP started", failingReceiver.StopReceiver.ReadStarted(), 250*time.Microsecond)

		stopDone := make(chan struct{})
		go func() {
			p.Stop()
			close(stopDone)
		}()

		waitForSignal(t, "receiver Stop called", failingReceiver.StopReceiver.StopCalled(), 250*time.Microsecond)

		select {
		case <-stopDone:
			t.Fatal("Stop() returned before reader gorutiness were released")
		case <-time.After(40 * time.Millisecond):
			// expected: Stop should still be blocked
		}

		track.Release()
		failingReceiver.StopReceiver.Release()

		select {
		case <-stopDone:
			// expected: Stop returned once readers were released
		case <-time.After(250 * time.Millisecond):
			t.Fatal("Stop() did not return after readers were released")
		}

		if got := failingReceiver.StopReceiver.StopCalls(); got != 1 {
			t.Fatalf("expected receiver.Stop() to be called once got %d", got)
		}
	})
}

func TestTrackPipeline_Render(t *testing.T) {
	t.Run("renders nothing when buffer is empty", func(t *testing.T) {
		p := &trackPipeline{
			track:       fakeTrack{id: "t1"},
			buffer:      noopBuffer{},
			logger:      logging.NewNullLogger("test", "null"),
			depaketizer: &fakeDepaketizer{},
			decoder:     &fakeDecoder{},
		}

		p.Render(time.Now())

		if got := p.depaketizer.(*fakeDepaketizer).depaketizeCount; got != 0 {
			t.Fatalf("expected Depaketize not to be called, got %d", got)
		}

		if got := p.decoder.(*fakeDecoder).acceptCount; got != 0 {
			t.Fatalf("expected Decoder.Accept not to be called, got %d", got)
		}
	})

	t.Run("depaketizes and decodes packets from buffer", func(t *testing.T) {
		p := &trackPipeline{
			track:       fakeTrack{id: "t1", ssrc: 1234},
			buffer:      &FakeBuffer{},
			logger:      logging.NewNullLogger("test", "null"),
			depaketizer: &fakeDepaketizer{},
			decoder:     &fakeDecoder{},
		}

		// Push a few packets into the buffer
		p.buffer.Push(1234, packet.Packet{RtpTs: 5678})
		p.buffer.Push(1234, packet.Packet{RtpTs: 6789})
		p.buffer.Push(1234, packet.Packet{RtpTs: 7890})

		p.Render(time.Now())

		if got := p.depaketizer.(*fakeDepaketizer).depaketizeCount; got != 3 {
			t.Fatalf("expected Depaketize to be called three times, got %d", got)
		}

		if got := p.decoder.(*fakeDecoder).acceptCount; got != 3 {
			t.Fatalf("expected Decoder.Accept to be called three times, got %d", got)
		}
	})

	t.Run("continues rendering packets even if depaketizer returns an error", func(t *testing.T) {
		fd := &fakeDepaketizer{}
		p := &trackPipeline{
			track:       fakeTrack{id: "t1", ssrc: 1234},
			buffer:      &FakeBuffer{},
			logger:      logging.NewNullLogger("test", "null"),
			depaketizer: fd,
			decoder:     &fakeDecoder{},
		}

		p.buffer.Push(1234, packet.Packet{RtpTs: 5678})
		p.buffer.Push(1234, packet.Packet{RtpTs: 6789})

		// Make the depaketizer return an error on the second packet
		fd.depaketizeFn = func(pkt packet.Packet) (depaketizers.EncodedUnit, error) {
			if fd.depaketizeCount == 1 {
				return nil, errors.New("depaketize error")
			}
			return nil, nil
		}

		p.Render(time.Now())

		if got := fd.depaketizeCount; got != 2 {
			t.Fatalf("expected Depaketize to be called once (including the error case), got %d", got)
		}

		if got := p.decoder.(*fakeDecoder).acceptCount; got != 1 {
			t.Fatalf("expected Decoder.Accept to be called twice (only for successful depaketizations), got %d", got)
		}
	})

	t.Run("continues rendering packets even if decoder returns an error", func(t *testing.T) {
		fd := &fakeDepaketizer{}
		fdec := &fakeDecoder{}
		p := &trackPipeline{
			track:       fakeTrack{id: "t1", ssrc: 1234},
			buffer:      &FakeBuffer{},
			logger:      logging.NewNullLogger("test", "null"),
			depaketizer: fd,
			decoder:     fdec,
		}

		p.buffer.Push(1234, packet.Packet{RtpTs: 5678})
		p.buffer.Push(1234, packet.Packet{RtpTs: 6789})

		fdec.acceptCount = 0
		fdec.acceptFn = func(depaketizers.EncodedUnit) error {
			if fdec.acceptCount == 1 {
				return errors.New("decode error")
			}
			return nil
		}

		p.Render(time.Now())

		if got := fd.depaketizeCount; got != 2 {
			t.Fatalf("expected Depaketize to be called twice, got %d", got)
		}

		if got := fdec.acceptCount; got != 2 {
			t.Fatalf("expected Decoder.Accept to be called twice (including the error case), got %d", got)
		}
	})
}

func TestTrackPipeline_onRTP(t *testing.T) {
	t.Run("passes exact packet and SSRC to buffer", func(t *testing.T) {
		buf := &recordingBuffer{status: jitter_buffer.StatusAccepted}
		p := &trackPipeline{
			track:  fakeTrack{id: "t1", ssrc: 1234},
			buffer: buf,
			logger: logging.NewNullLogger("test", "null"),
		}
		pkt := packet.Packet{
			RtpTs:   5678,
			Seq:     42,
			Payload: packet.PacketPayload{1, 2, 3},
		}

		if err := p.onRTP(pkt); err != nil {
			t.Fatalf("onRTP() error = %v", err)
		}

		if buf.calls != 1 {
			t.Fatalf("expected buffer Push to be called once, got %d", buf.calls)
		}
		if buf.ssrc != 1234 {
			t.Fatalf("expected SSRC 1234, got %d", buf.ssrc)
		}
		if buf.pkt.RtpTs != pkt.RtpTs || buf.pkt.Seq != pkt.Seq || !slices.Equal(buf.pkt.Payload, pkt.Payload) {
			t.Fatalf("buffer got packet %+v, want %+v", buf.pkt, pkt)
		}
	})

	t.Run("propagates buffer errors", func(t *testing.T) {
		wantErr := errors.New("buffer push failed")
		buf := &recordingBuffer{
			status: jitter_buffer.StatusAccepted,
			err:    wantErr,
		}
		p := &trackPipeline{
			track:  fakeTrack{id: "t1", ssrc: 1234},
			buffer: buf,
			logger: logging.NewNullLogger("test", "null"),
		}

		err := p.onRTP(packet.Packet{RtpTs: 5678})
		if !errors.Is(err, wantErr) {
			t.Fatalf("onRTP() error = %v, want %v", err, wantErr)
		}
	})
}

func TestTrackPipeline_HandleRTP(t *testing.T) {
	t.Run("deadline error", func(t *testing.T) {
		wantErr := errors.New("set deadline failed")
		track := NewHandleRTPTrack("t1", packet.Packet{}, wantErr, nil)
		p := &trackPipeline{
			track:  track,
			buffer: noopBuffer{},
			config: TrackPipelineConfig{RTPReadDeadline: 10 * time.Millisecond},
			logger: logging.NewNullLogger("test", "null"),
		}

		err := p.handleRTP()
		if !errors.Is(err, wantErr) {
			t.Fatalf("handleRTP() error = %v, want %v", err, wantErr)
		}
		if got := track.ReadCalls(); got != 0 {
			t.Fatalf("expected ReadRTP not to be called on deadline error, got %d", got)
		}
	})

	t.Run("read error", func(t *testing.T) {
		wantErr := errors.New("read rtp failed")
		track := NewHandleRTPTrack("t1", packet.Packet{}, nil, wantErr)
		p := &trackPipeline{
			track:  track,
			buffer: noopBuffer{},
			config: TrackPipelineConfig{RTPReadDeadline: 10 * time.Millisecond},
			logger: logging.NewNullLogger("test", "null"),
		}

		err := p.handleRTP()
		if !errors.Is(err, wantErr) {
			t.Fatalf("handleRTP() error = %v, want %v", err, wantErr)
		}
		if got := track.ReadCalls(); got != 1 {
			t.Fatalf("expected ReadRTP to be called once, got %d", got)
		}
	})

	t.Run("success path", func(t *testing.T) {
		track := NewHandleRTPTrack("t1", packet.Packet{}, nil, nil)
		p := &trackPipeline{
			track:  track,
			buffer: noopBuffer{},
			config: TrackPipelineConfig{RTPReadDeadline: 10 * time.Millisecond},
			logger: logging.NewNullLogger("test", "null"),
		}

		if err := p.handleRTP(); err != nil {
			t.Fatalf("handleRTP() error = %v, want nil", err)
		}
		if got := track.SetDeadlineCalls(); got != 1 {
			t.Fatalf("expected SetReadDeadline to be called once, got %d", got)
		}
		if got := track.ReadCalls(); got != 1 {
			t.Fatalf("expected ReadRTP to be called once, got %d", got)
		}
	})
}

func TestTrackPipeline_HandleRTCP(t *testing.T) {
	t.Run("deadline error", func(t *testing.T) {
		wantErr := errors.New("set deadline failed")
		receiver := NewControllableRTPReceiver(nil, wantErr, nil)
		p := &trackPipeline{
			track:    fakeTrack{id: "t1"},
			receiver: receiver,
			config:   TrackPipelineConfig{RTCPReadDeadline: 10 * time.Millisecond},
			logger:   logging.NewNullLogger("test", "null"),
		}

		err := p.handleRTCP()
		if !errors.Is(err, wantErr) {
			t.Fatalf("handleRTCP() error = %v, want %v", err, wantErr)
		}
		if got := receiver.ReadCalls(); got != 0 {
			t.Fatalf("expected ReadRTCP not to be called on deadline error, got %d", got)
		}
	})

	t.Run("read error", func(t *testing.T) {
		wantErr := errors.New("read rtcp failed")
		receiver := NewControllableRTPReceiver(nil, nil, wantErr)
		p := &trackPipeline{
			track:    fakeTrack{id: "t1"},
			receiver: receiver,
			config:   TrackPipelineConfig{RTCPReadDeadline: 10 * time.Millisecond},
			logger:   logging.NewNullLogger("test", "null"),
		}

		err := p.handleRTCP()
		if !errors.Is(err, wantErr) {
			t.Fatalf("handleRTCP() error = %v, want %v", err, wantErr)
		}
		if got := receiver.ReadCalls(); got != 1 {
			t.Fatalf("expected ReadRTCP to be called once, got %d", got)
		}
	})

	t.Run("success path", func(t *testing.T) {
		receiver := NewControllableRTPReceiver(nil, nil, nil)
		p := &trackPipeline{
			track:    fakeTrack{id: "t1"},
			receiver: receiver,
			config:   TrackPipelineConfig{RTCPReadDeadline: 10 * time.Millisecond},
			logger:   logging.NewNullLogger("test", "null"),
		}

		if err := p.handleRTCP(); err != nil {
			t.Fatalf("handleRTCP() error = %v, want nil", err)
		}
		if got := receiver.SetDeadlineCalls(); got != 1 {
			t.Fatalf("expected SetReadDeadline to be called once, got %d", got)
		}
		if got := receiver.ReadCalls(); got != 1 {
			t.Fatalf("expected ReadRTCP to be called once, got %d", got)
		}
	})
}

func waitForReadCall(t *testing.T, name string, reads func() int32, n int32) {
	t.Helper()

	waitUntil := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(waitUntil) {
		if reads() >= n {
			return
		}
		time.Sleep(1 * time.Millisecond)
	}

	t.Fatalf("expected %s to be called at least %d times, got %d", name, n, reads())
}

func waitForSignal(t *testing.T, name string, ch <-chan struct{}, timeout time.Duration) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for %s", name)
	}
}
