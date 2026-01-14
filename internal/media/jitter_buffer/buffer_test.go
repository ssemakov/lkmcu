package jitter_buffer

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/ssemakov/lkmcu/internal/logging"
	"github.com/ssemakov/lkmcu/internal/media/clock"
	"github.com/ssemakov/lkmcu/internal/metrics"
	"github.com/ssemakov/lkmcu/internal/testutil"
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

func TestBuffer_Push_Accepted_Before_SR(t *testing.T) {
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

func TestBuffer_Push_Accepted_After_SR(t *testing.T) {
	const (
		ssrc      = uint32(0xdeadbeef)
		clockRate = uint32(90000)
	)
	buf, m := newTestBuffer()
	m.SetClockRate(ssrc, clockRate)

	baseRTP, hi, lo, baseNTP, _ := testutil.MakeSRBase(clockRate, 100*time.Millisecond)

	if err := m.UpdateFromSR(ssrc, baseRTP, hi, lo); err != nil {
		t.Fatal(err)
	}

	p1RTP := testutil.RTPAfter(clockRate, baseRTP, 50*time.Millisecond)
	p1 := Packet{Seq: 1, RtpTs: p1RTP}

	status, err := buf.Push(ssrc, p1)

	if err != nil {
		t.Fatalf("no error expected, got %v", err)
	}

	// Mapper returns StatusMapperDegraded after single SR and normalizes after the second one
	if status != StatusMapperDegraded {
		t.Fatalf("want StatusMapperDegraded, got %v", StatusString(status))
	}

	baseRTP, hi, lo, _ = testutil.MakeNextSR(baseRTP, baseNTP, clockRate, 1*time.Second)
	if err := m.UpdateFromSR(ssrc, baseRTP, hi, lo); err != nil {
		t.Fatal(err)
	}

	p2RTP := testutil.RTPAfter(clockRate, baseRTP, 50*time.Millisecond)
	p2 := Packet{Seq: 2, RtpTs: p2RTP}

	status, err = buf.Push(ssrc, p2)

	if err != nil {
		t.Fatalf("no error expected, got %v", err)
	}

	if status != StatusAccepted {
		t.Fatalf("want StatusAccepted, got %v", StatusString(status))
	}
}

func TestBuffer_Push_Duplicate(t *testing.T) {
	const (
		ssrc      = uint32(0xdeadbeef)
		clockRate = uint32(90000)
	)

	buf, m := newTestBuffer()
	m.SetClockRate(ssrc, clockRate)

	baseRTP, hi, lo, _, _ := testutil.MakeSRBase(clockRate, 100*time.Millisecond)

	if err := m.UpdateFromSR(ssrc, baseRTP, hi, lo); err != nil {
		t.Fatal(err)
	}

	p1RTP := testutil.RTPAfter(clockRate, baseRTP, 50*time.Millisecond)
	p1 := Packet{Seq: 1, RtpTs: p1RTP}

	_, err := buf.Push(ssrc, p1)

	if err != nil {
		t.Fatalf("no error expected, got %v", err)
	}

	p2 := Packet{Seq: 1, RtpTs: p1RTP}
	status, err := buf.Push(ssrc, p2)

	if err != nil {
		t.Fatalf("no error expected, got %v", err)
	}

	if status != StatusDuplicate {
		t.Fatalf("want StatusDuplicate, got %v", StatusString(status))
	}
}

func TestBuffer_Push_LateDrop(t *testing.T) {
	const (
		ssrc      = uint32(0xdeadbeef)
		clockRate = uint32(90000)
	)

	buf, m := newTestBuffer()
	m.SetClockRate(ssrc, clockRate)

	baseRTP, hi, lo, _, _ := testutil.MakeSRBase(clockRate, 6*time.Second)

	if err := m.UpdateFromSR(ssrc, baseRTP, hi, lo); err != nil {
		t.Fatal(err)
	}

	p1RTP := testutil.RTPAfter(clockRate, baseRTP, 50*time.Millisecond)
	p1 := Packet{Seq: 1, RtpTs: p1RTP}

	status, err := buf.Push(ssrc, p1)

	if err != nil {
		t.Fatalf("no error expected, got %v", err)
	}

	if status != StatusLateDrop {
		t.Fatalf("want StatusLateDrop, got %v", StatusString(status))
	}

}

func TestBuffer_Push_MapperDegraded(t *testing.T) {
	const (
		ssrc      = uint32(0xdeadbeef)
		clockRate = uint32(90000)
	)
	buf, m := newTestBuffer()
	m.SetClockRate(ssrc, clockRate)

	baseRTP, hi, lo, _, _ := testutil.MakeSRBase(clockRate, 100*time.Millisecond)

	if err := m.UpdateFromSR(ssrc, baseRTP, hi, lo); err != nil {
		t.Fatal(err)
	}

	p1RTP := testutil.RTPAfter(clockRate, baseRTP, 50*time.Millisecond)
	p1 := Packet{Seq: 1, RtpTs: p1RTP}

	status, err := buf.Push(ssrc, p1)

	if err != nil {
		t.Fatalf("no error expected, got %v", err)
	}

	// Mapper returns StatusMapperDegraded after single SR and normalizes after the second one
	if status != StatusMapperDegraded {
		t.Fatalf("want StatusMapperDegraded, got %v", StatusString(status))
	}

	target := buf.Target(ssrc)

	if target != buf.Config().MaxTarget {
		t.Fatalf("want Target to be MaxTarget %v, got %v", buf.Config().MaxTarget, target)
	}
}

func TestBuffer_Push_MapperError(t *testing.T) {
	const (
		ssrc      = uint32(0xdeadbeef)
		clockRate = uint32(90000)
	)
	buf, _ := newTestBuffer()

	baseRTP, _, _, _, _ := testutil.MakeSRBase(clockRate, 100*time.Millisecond)

	p1RTP := testutil.RTPAfter(clockRate, baseRTP, 50*time.Millisecond)
	p1 := Packet{Seq: 1, RtpTs: p1RTP}

	status, err := buf.Push(ssrc, p1)

	if status != StatusMapperError {
		t.Fatalf("want StatusMapperError, got %v", StatusString(status))
	}

	if err == nil {
		t.Fatalf("want an error, got nil")
	}
}

func TestBuffer_Push_PacketsRearrangedAfterSR(t *testing.T) {
	const (
		ssrc      = uint32(0xdeadbeef)
		clockRate = uint32(90000)
	)
	buf, m := newTestBuffer()
	m.SetClockRate(ssrc, clockRate)

	// Initial SR at t0 = now - 100ms
	baseRTP, hi, lo, baseNTP, _ := testutil.MakeSRBase(clockRate, 100*time.Millisecond)

	// Push packets in reverse order
	// To avoid mapper discontinuity times must be less than 10ms apart per BackstepClampThreshold default config
	deltas := []time.Duration{25, 20, 15, 10} // in milliseconds

	// Packets pushed before SR:
	// d=25ms (Seq 0) -> PTS = t0 + 25ms = now - 75ms
	// d=20ms (Seq 1) -> PTS = t0 + 20ms = now - 80ms
	// d=15ms (Seq 2) -> PTS = t0 + 15ms = now - 85ms
	// d=10ms (Seq 3) -> PTS = t0 + 10ms = now - 90ms
	for i, d := range deltas {
		pRTP := testutil.RTPAfter(clockRate, baseRTP, d*time.Millisecond)
		p := Packet{Seq: uint16(i), RtpTs: pRTP}
		buf.Push(ssrc, p)
	}

	// Pop should not return any packets before SR
	pkt, ok := buf.Pop(ssrc, time.Now())
	if ok {
		t.Fatalf("expected no packet before SR, got %+v", pkt)
	}

	// Update mapper with second SR to normalize state
	// new SR at t1 = t0 + 10µs (~now - 100ms + 10µs). 10µs for test purposes to reset mapper state without shifting time significantly.
	baseRTP, hi, lo, baseNTP = testutil.MakeNextSR(baseRTP, baseNTP, clockRate, 10*time.Microsecond)

	if err := m.UpdateFromSR(ssrc, baseRTP, hi, lo); err != nil {
		t.Fatal(err)
	}

	// push one more packet after SR to trigger recalculation
	// PTS = t1 + 30ms = now - 70ms
	p := Packet{Seq: 99, RtpTs: testutil.RTPAfter(clockRate, baseRTP, 30*time.Millisecond)}
	buf.Push(ssrc, p)

	// Normalize mapper state out of the discontinuity with second SR
	/// new SR at t2 = t1 + 50µs (~now - 100ms + 60µs)
	baseRTP, hi, lo, baseNTP = testutil.MakeNextSR(baseRTP, baseNTP, clockRate, 50*time.Microsecond)
	if err := m.UpdateFromSR(ssrc, baseRTP, hi, lo); err != nil {
		t.Fatal(err)
	}

	// PTS = t2 + 35ms = now - 65ms
	p = Packet{Seq: 100, RtpTs: testutil.RTPAfter(clockRate, baseRTP, 35*time.Millisecond)}
	buf.Push(ssrc, p)

	now := time.Now()

	// After SR Pop should return packets in order according to to their calculated PTS NTP
	for i := len(deltas) - 1; i >= 0; i-- {
		pkt, ok := buf.Pop(ssrc, now)
		now = now.Add(5 * time.Millisecond) // advance time to allow popping next packet
		if !ok {
			t.Fatalf("expected packet after SR, got none")
		}
		expectedSeq := uint16(i)
		if pkt.Seq != expectedSeq {
			t.Fatalf("expected packet with Seq %d, got %d", expectedSeq, pkt.Seq)
		}
	}
}

func TestBuffer_Pop_HoldsPacketsUntilSRArrives(t *testing.T) {
	t.Skip("TODO: implement (pop holds packets until SR Arrives)")
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
