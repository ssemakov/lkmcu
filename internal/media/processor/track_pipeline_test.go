package processor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ssemakov/lkmcu/internal/logging"
	"github.com/ssemakov/lkmcu/internal/testutil"
)

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
		Ctx: context.Background(),
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

func TestTrackPipeline_StartLoops(t *testing.T) {
	t.Run("rtp loop continues on timeout", func(t *testing.T) {
		ctx := context.Background()
		track, receiver, err := testutil.MakeRTPTrackAndReceiver(
			testutil.TimeoutRTPTrack,
			testutil.EOFReceiver,
		)
		if err != nil {
			t.Fatalf("MakeTimeoutRTPTrackPipeline() error = %v", err)
		}

		factory := &DefaultTrackPipelineFactory{
			Ctx:    ctx,
			Config: DefaultTrackPipelineConfig(),
			Logger: logging.NewNullLogger("test", "null"),
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
		track, receiver, err := testutil.MakeRTPTrackAndReceiver(
			testutil.TimeoutRTPTrack,
			testutil.TimeoutReceiver,
		)
		if err != nil {
			t.Fatalf("MakeTimeoutRTPTrackPipeline() error = %v", err)
		}

		factory := &DefaultTrackPipelineFactory{
			Ctx:    ctx,
			Config: DefaultTrackPipelineConfig(),
			Logger: logging.NewNullLogger("test", "null"),
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
			trackType    testutil.TestTrack
			receiverType testutil.TestReceiver
		}{
			{
				name:         "io.EOF",
				trackType:    testutil.EOFRTPTrack,
				receiverType: testutil.EOFReceiver,
			},
			{
				name:         "io.ErrClosedPipe",
				trackType:    testutil.ClosedPipeRTPTrack,
				receiverType: testutil.ClosedPipeReceiver,
			},
			{
				name:         "context.Canceled",
				trackType:    testutil.CanceledRTPTrack,
				receiverType: testutil.CanceledReceiver,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				track, receiver, err := testutil.MakeRTPTrackAndReceiver(
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
		track, receiver, err := testutil.MakeRTPTrackAndReceiver(
			testutil.UnexpectedErrRTPTrack,
			testutil.UnexpectedErrReceiver,
		)
		if err != nil {
			t.Fatalf("MakeRTPTrackAndReceiver() error = %v", err)
		}

		factory := &DefaultTrackPipelineFactory{
			Ctx:    context.Background(),
			Config: DefaultTrackPipelineConfig(),
			Logger: logging.NewNullLogger("test", "null"),
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
		track, receiver := testutil.MakeStopTrackAndReceiver(
			true,
		)

		factory := &DefaultTrackPipelineFactory{
			Ctx: context.Background(),
			Config: TrackPipelineConfig{
				RTPReadDeadline:  10 * time.Second,
				RTCPReadDeadline: 10 * time.Second,
			},
			Logger: logging.NewNullLogger("test", "null"),
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
		track, receiver := testutil.MakeStopTrackAndReceiver(
			false,
		)

		factory := &DefaultTrackPipelineFactory{
			Ctx: context.Background(),
			Config: TrackPipelineConfig{
				RTPReadDeadline:  10 * time.Second,
				RTCPReadDeadline: 10 * time.Second,
			},
			Logger: logging.NewNullLogger("test", "null"),
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
}

func TestTrackPipeline_HandleRTP_Stubs(t *testing.T) {
	t.Run("deadline error", func(t *testing.T) {
		t.Skip("TODO: verify handleRTP returns SetReadDeadline error")
	})

	t.Run("read error", func(t *testing.T) {
		t.Skip("TODO: verify handleRTP returns ReadRTP error")
	})

	t.Run("onRTP error", func(t *testing.T) {
		t.Skip("TODO: verify handleRTP returns onRTP error")
	})

	t.Run("success path", func(t *testing.T) {
		t.Skip("TODO: verify handleRTP succeeds when deadline/read/onRTP succeed")
	})
}

func TestTrackPipeline_HandleRTCP_Stubs(t *testing.T) {
	t.Run("deadline error", func(t *testing.T) {
		t.Skip("TODO: verify handleRTCP returns SetReadDeadline error")
	})

	t.Run("read error", func(t *testing.T) {
		t.Skip("TODO: verify handleRTCP returns ReadRTCP error")
	})

	t.Run("onRTCP error", func(t *testing.T) {
		t.Skip("TODO: verify handleRTCP returns onRTCP error")
	})

	t.Run("success path", func(t *testing.T) {
		t.Skip("TODO: verify handleRTCP succeeds when deadline/read/onRTCP succeed")
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
