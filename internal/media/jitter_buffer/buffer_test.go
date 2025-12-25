package jitter_buffer

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/ssemakov/lkmcu/internal/logging"
	"github.com/ssemakov/lkmcu/internal/media/clock"
	"github.com/ssemakov/lkmcu/internal/metrics"
)

// Compile-time check: jitterBuffer implements Buffer.
var _ Buffer = (*jitterBuffer)(nil)

func TestBuffer_NewBuffer_Defaults(t *testing.T) {
	m := clock.ClockMapper{}
	logger := logging.NewNullLogger("east", "info")
	buf := NewBuffer(&m, logger, nil, nil)

	if buf == nil {
		t.Fatal("NewBuffer returned nil")
	}

	got := buf.Config()
	want := DefaultConfig()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("config mismatch (-want +got):\n%s", diff)
	}
}

func TestBuffer_NewBuffer_DupeWindowConstraint(t *testing.T) {
	m := clock.ClockMapper{}
	cfg := Config{
		DupeBufferSize:  10,
		MaxReorderDepth: 100,
	}
	logger := logging.NewNullLogger("east", "info")

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when DupeBufferSize less than MaxReorderDepth")
		}
	}()

	NewBuffer(&m, logger, &cfg, nil)
}

func newTestBuffer() (Buffer, *clock.ClockMapper) {
	// logger := logging.NewNullLogger("test", "debug")
	logger := logging.NewConsoleLogger("test", "debug")
	mCfg := clock.DefaultClockMapperConfig()
	m := clock.NewClockMapper(metrics.NewMetrics(), logger, mCfg)

	buf := NewBuffer(m, logger, nil, nil)

	return buf, m
}

func TestBuffer_Push_Accepted(t *testing.T) {
	const (
		ssrc      = uint32(0xdeadbeef)
		clockRate = uint32(90000)
	)
	buf, m := newTestBuffer()
	m.SetClockRate(ssrc, clockRate)

	packet := Packet{Seq: 1, PayloadTs: time.Unix(0, 50)}

	status, err := buf.Push(ssrc, packet)

	if err != nil {
		t.Fatalf("no error expected, got %v", err)
	}

	if status != StatusAccepted {
		t.Fatalf("want StatusAccepted, got %v", StatusString(status))
	}
}

func TestBuffer_Push_Duplicate(t *testing.T) {
	t.Skip("TODO: implement (drop duplicate by {pts, seq})")
}

func TestBuffer_Push_LateDrop(t *testing.T) {
	t.Skip("TODO: implement (drop too-late packet)")
}

func TestBuffer_Push_MapperDegraded(t *testing.T) {
	t.Skip("TODO: implement (mapper degraded widens target and returns status)")
}

func TestBuffer_Push_MapperError(t *testing.T) {
	t.Skip("TODO: implement (mapper error returns StatusMapperError)")
}

func TestBuffer_Pop_ReadyAndOrder(t *testing.T) {
	t.Skip("TODO: implement (pop when pts <= cursor, stable ordering)")
}

func TestBuffer_Pop_UnderflowFreezeAndHandler(t *testing.T) {
	t.Skip("TODO: implement (freeze cursor on underflow; invoke handler)")
}

func TestBuffer_Occupancy_Report(t *testing.T) {
	t.Skip("TODO: implement (occupancy reflects newest-oldest span)")
}

func TestBuffer_Target_NominalAndPerSSRC(t *testing.T) {
	t.Skip("TODO: implement (nominal target for unknown SSRC; per-SSRC target otherwise)")
}

func TestSSRCBuffer_UpdateOccupancy(t *testing.T) {
	t.Skip("TODO: implement (updates occupy based on heap oldest/newest)")
}

func TestSSRCBuffer_Cursor_FrozenAndWide(t *testing.T) {
	t.Skip("TODO: implement (cursor frozen during rebuffer; otherwise now - target)")
}

func TestBuffer_DropsOldest_OverMaxTarget(t *testing.T) {
	t.Skip("TODO: implement (cap latency by dropping oldest when occupancy exceeds target)")
}
