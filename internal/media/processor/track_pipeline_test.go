package processor

import (
	"context"
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
		track, receiver, trackCounter, _, err := testutil.MakeRTPTrackAndReceiver(
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

		waitUntil := time.Now().Add(250 * time.Millisecond)
		for time.Now().Before(waitUntil) {
			if trackCounter.ReadCalls() >= 2 {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}

		t.Fatalf("expected ReadRTP to be called at least twice, got %d", trackCounter.ReadCalls())
	})

	t.Run("rtcp loop continues on timeout", func(t *testing.T) {
		ctx := context.Background()
		track, receiver, _, receiverCounter, err := testutil.MakeRTPTrackAndReceiver(
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

		waitUntil := time.Now().Add(250 * time.Millisecond)
		for time.Now().Before(waitUntil) {
			if receiverCounter.ReadCalls() >= 2 {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}

		t.Fatalf("expected ReadRTCP to be called at least twice, got %d", receiverCounter.ReadCalls())
	})

	t.Run("loops stop on expected closure errors", func(t *testing.T) {
		t.Skip("TODO: verify Start loops stop on io.EOF/io.ErrClosedPipe/context.Canceled")
	})

	t.Run("loops stop on unexpected errors", func(t *testing.T) {
		t.Skip("TODO: verify Start loops stop when handlers return unexpected errors")
	})
}

func TestTrackPipeline_Stop_Stubs(t *testing.T) {
	t.Run("cancels context and unblocks readers", func(t *testing.T) {
		t.Skip("TODO: verify Stop cancels context and sets read deadlines")
	})

	t.Run("stops receiver and waits for goroutines", func(t *testing.T) {
		t.Skip("TODO: verify Stop calls receiver.Stop and waits for Start goroutines to exit")
	})

	t.Run("logs recoverable stop path errors", func(t *testing.T) {
		t.Skip("TODO: verify Stop logs deadline/receiver errors without panicking")
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

func TestTrackPipeline_IsTimeout_Stubs(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		t.Skip("TODO: verify isTimeout(nil) == false")
	})

	t.Run("packetio timeout", func(t *testing.T) {
		t.Skip("TODO: verify isTimeout(packetio.ErrTimeout) == true")
	})

	t.Run("net timeout error", func(t *testing.T) {
		t.Skip("TODO: verify isTimeout(net.Error{Timeout:true}) == true")
	})

	t.Run("non-timeout error", func(t *testing.T) {
		t.Skip("TODO: verify isTimeout(non-timeout error) == false")
	})
}
