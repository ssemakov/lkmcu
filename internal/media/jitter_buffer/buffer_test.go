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

func newTestBuffer(cfg *Config, onUnderflow OnUnderflowHandler) (Buffer, *clock.ClockMapper) {
	// logger := logging.NewNullLogger("test", "debug")
	logger := logging.NewConsoleLogger("test", "debug")
	mCfg := clock.DefaultClockMapperConfig()
	m := clock.NewClockMapper(metrics.NewMetrics(), logger, mCfg)

	buf := NewBuffer(m, logger, cfg, onUnderflow)

	return buf, m
}

func TestBuffer_Push_Accepted_Before_SR(t *testing.T) {
	const (
		ssrc      = uint32(0xdeadbeef)
		clockRate = uint32(90000)
	)
	buf, m := newTestBuffer(nil, nil)
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
	buf, m := newTestBuffer(nil, nil)
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

	buf, m := newTestBuffer(nil, nil)
	m.SetClockRate(ssrc, clockRate)

	baseRTP, hi, lo, _, _ := testutil.MakeSRBase(clockRate, 10*time.Millisecond)

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

	buf, m := newTestBuffer(nil, nil)
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
	buf, m := newTestBuffer(nil, nil)
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

func TestBuffer_Push_PacketsRearrangedAfterSR(t *testing.T) {
	const (
		ssrc      = uint32(0xdeadbeef)
		clockRate = uint32(90000)
	)
	buf, m := newTestBuffer(nil, nil)
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
	const (
		ssrc      = uint32(0xdeadbeef)
		clockRate = uint32(90000)
	)
	buf, m := newTestBuffer(nil, nil)
	m.SetClockRate(ssrc, clockRate)

	baseRTP, hi, lo, _, _ := testutil.MakeSRBase(clockRate, 100*time.Millisecond)

	// Push packets before any SR
	p1 := Packet{Seq: 1, RtpTs: testutil.RTPAfter(clockRate, baseRTP, 15*time.Millisecond)}
	buf.Push(ssrc, p1)

	pkt, ok := buf.Pop(ssrc, time.Now())
	if ok {
		t.Fatalf("expected no packet before SR, got %+v", pkt)
	}

	// Now update SR
	if err := m.UpdateFromSR(ssrc, baseRTP, hi, lo); err != nil {
		t.Fatal(err)
	}

	// Push another packet after SR to trigger recalculation
	p2 := Packet{Seq: 2, RtpTs: testutil.RTPAfter(clockRate, baseRTP, 20*time.Millisecond)}
	buf.Push(ssrc, p2)

	// Advance time beyond max target to allow popping (the window is now widened to 200ms as result of single SR and discontinuity)
	now := time.Now().Add(120 * time.Millisecond)
	// Now Pop should return packets
	pkt, ok = buf.Pop(ssrc, now)
	if !ok {
		t.Fatalf("expected packet after SR, got none")
	}

	if pkt.Seq != 1 {
		t.Fatalf("expected packet with Seq 1, got %d", pkt.Seq)
	}

	pkt, ok = buf.Pop(ssrc, now)
	if !ok {
		t.Fatalf("expected second packet after SR, got none")
	}

	if pkt.Seq != 2 {
		t.Fatalf("expected packet with Seq 2, got %d", pkt.Seq)
	}
}

func TestBuffer_Pop_UnderflowFreezeAndHandler(t *testing.T) {
	const (
		ssrc      = uint32(0xdeadbeef)
		clockRate = uint32(90000)
	)

	ch := make(chan uint32, 1)
	handler := func(ssrc uint32) { ch <- ssrc }

	buf, m := newTestBuffer(nil, handler)
	m.SetClockRate(ssrc, clockRate)

	baseRTP, hi, lo, baseNTP, _ := testutil.MakeSRBase(clockRate, 100*time.Millisecond)

	if err := m.UpdateFromSR(ssrc, baseRTP, hi, lo); err != nil {
		t.Fatal(err)
	}

	// Second SR to normalize mapper state
	baseRTP, hi, lo, _ = testutil.MakeNextSR(baseRTP, baseNTP, clockRate, 10*time.Microsecond)
	if err := m.UpdateFromSR(ssrc, baseRTP, hi, lo); err != nil {
		t.Fatal(err)
	}

	// Push a packet
	p1 := Packet{Seq: 1, RtpTs: testutil.RTPAfter(clockRate, baseRTP, 10*time.Millisecond)}
	buf.Push(ssrc, p1)

	// Should pop normally
	now := time.Now().Add(20 * time.Millisecond)
	pkt, ok := buf.Pop(ssrc, now)
	if !ok {
		t.Fatalf("expected packet, got none")
	}

	if pkt.Seq != 1 {
		t.Fatalf("expected packet with Seq 1, got %d", pkt.Seq)
	}

	// Move time forward above the underflow guard threshold
	now = now.Add(buf.Config().UndeflowGuard + 10*time.Millisecond)
	// Next Pop should underflow
	pkt, ok = buf.Pop(ssrc, now)
	if ok {
		t.Fatalf("expected no packet (underflow), got %+v", pkt)
	}

	select {
	case gotSsrc := <-ch:
		if gotSsrc != ssrc {
			t.Fatalf("got ssrc %x, want %x", gotSsrc, ssrc)
		}
	default:
		t.Fatal("expected underflow handler to be called, but it was not")
	}

	// Further Pops within the underflow freeze period should not trigger the handler again
	pkt, ok = buf.Pop(ssrc, now.Add(10*time.Millisecond))
	if ok {
		t.Fatalf("expected no packet (underflow), got %+v", pkt)
	}

	select {
	case gotSsrc := <-ch:
		t.Fatalf("underflow handler called again unexpectedly with ssrc %x", gotSsrc)
	default:
		// expected
	}
}

func TestBuffer_Occupancy_Report(t *testing.T) {
	const (
		ssrc      = uint32(0xdeadbeef)
		clockRate = uint32(90000)
	)

	buf, m := newTestBuffer(nil, nil)
	m.SetClockRate(ssrc, clockRate)

	baseRTP, hi, lo, _, _ := testutil.MakeSRBase(clockRate, 100*time.Millisecond)

	if err := m.UpdateFromSR(ssrc, baseRTP, hi, lo); err != nil {
		t.Fatal(err)
	}

	// Push packets
	numPackets := 5
	for i := 0; i < numPackets; i++ {
		p := Packet{Seq: uint16(i), RtpTs: testutil.RTPAfter(clockRate, baseRTP, time.Duration(i)*10*time.Millisecond)}
		buf.Push(ssrc, p)
	}

	occ := buf.Occupancy(ssrc)
	expectedOcc := time.Duration(numPackets-1) * 10 * time.Millisecond

	if absDuration := occ - expectedOcc; absDuration < 0 {
		absDuration = -absDuration
		if absDuration > 100*time.Nanosecond {
			t.Fatalf("expected occupancy %d, got %d", expectedOcc, occ)
		}
	} else {
		if absDuration > 100*time.Nanosecond {
			t.Fatalf("expected occupancy %d, got %d", expectedOcc, occ)
		}
	}
}

func TestBuffer_Target_NominalAndPerSSRC(t *testing.T) {
	// t.Skip("TODO: implement (nominal target for unknown SSRC; per-SSRC target otherwise)")
	const (
		ssrc      = uint32(0xdeadbeef)
		clockRate = uint32(90000)
	)

	buf, m := newTestBuffer(nil, nil)
	m.SetClockRate(ssrc, clockRate)

	baseRTP, hi, lo, baseNTP, _ := testutil.MakeSRBase(clockRate, 100*time.Millisecond)

	if err := m.UpdateFromSR(ssrc, baseRTP, hi, lo); err != nil {
		t.Fatal(err)
	}

	nom := buf.Target(0xabadcafe) // unknown SSRC

	if nom != buf.Config().Target {
		t.Fatalf("want nominal Target %v, got %v", buf.Config().Target, nom)
	}

	p := Packet{Seq: 1, RtpTs: testutil.RTPAfter(clockRate, baseRTP, 10*time.Millisecond)}
	buf.Push(ssrc, p)

	// After first SR mapper is degraded, so target should be MaxTarget
	if buf.Target(ssrc) != buf.Config().MaxTarget {
		t.Fatalf("want per-SSRC Target %v, got %v", buf.Config().MaxTarget, buf.Target(ssrc))
	}

	// Second SR to normalize mapper state
	baseRTP, hi, lo, _ = testutil.MakeNextSR(baseRTP, baseNTP, clockRate, 10*time.Microsecond)
	if err := m.UpdateFromSR(ssrc, baseRTP, hi, lo); err != nil {
		t.Fatal(err)
	}

	p = Packet{Seq: 2, RtpTs: testutil.RTPAfter(clockRate, baseRTP, 20*time.Millisecond)}
	buf.Push(ssrc, p)

	// After second SR mapper is normalized, so target should be nominal
	if buf.Target(ssrc) != buf.Config().Target {
		t.Fatalf("after second SR want per-SSRC Target %v, got %v", buf.Config().Target, buf.Target(ssrc))
	}

}

func TestSSRCBuffer_UpdateOccupancy(t *testing.T) {
	base := time.Unix(0, 0)
	sb := SSRCBuffer{heap: &PacketHeap{}}

	sb.updateOccupancy()
	if sb.occupy != 0 {
		t.Fatalf("expected occupancy 0 for empty heap, got %v", sb.occupy)
	}

	sb.heap.PushPacket(&Packet{PayloadTs: base.Add(10 * time.Millisecond), Seq: 2})
	sb.updateOccupancy()
	if sb.occupy != 0 {
		t.Fatalf("expected occupancy 0 for single packet, got %v", sb.occupy)
	}

	sb.heap.PushPacket(&Packet{PayloadTs: base.Add(0), Seq: 10})
	sb.heap.PushPacket(&Packet{PayloadTs: base.Add(20 * time.Millisecond), Seq: 1})
	sb.updateOccupancy()

	expected := 20 * time.Millisecond
	if sb.occupy != expected {
		t.Fatalf("expected occupancy %v, got %v", expected, sb.occupy)
	}
}

func TestSSRCBuffer_Cursor_FrozenAndWide(t *testing.T) {
	base := time.Unix(0, 0)
	sb := SSRCBuffer{
		target:       50 * time.Millisecond,
		freezeUntil:  base.Add(100 * time.Millisecond),
		freezeCursor: base.Add(10 * time.Millisecond),
	}

	now := base.Add(50 * time.Millisecond)
	if got := sb.cursor(now); !got.Equal(sb.freezeCursor) {
		t.Fatalf("expected frozen cursor %v, got %v", sb.freezeCursor, got)
	}

	now = base.Add(120 * time.Millisecond)
	expected := now.Add(-sb.target)
	if got := sb.cursor(now); !got.Equal(expected) {
		t.Fatalf("expected wide cursor %v, got %v", expected, got)
	}
}

func TestBuffer_DropsOldest_OverMaxTarget(t *testing.T) {
	const (
		ssrc      = uint32(0xdeadbeef)
		clockRate = uint32(90000)
	)

	cfg := DefaultConfig()
	cfg.LateDrop = 500 * time.Millisecond
	// set target low to trigger drops quickly
	cfg.Target = 30 * time.Millisecond
	buf, m := newTestBuffer(cfg, nil)

	m.SetClockRate(ssrc, clockRate)
	baseRTP, hi, lo, baseNTP, _ := testutil.MakeSRBase(clockRate, 100*time.Millisecond)
	if err := m.UpdateFromSR(ssrc, baseRTP, hi, lo); err != nil {
		t.Fatal(err)
	}
	baseRTP, hi, lo, _ = testutil.MakeNextSR(baseRTP, baseNTP, clockRate, 10*time.Microsecond)
	if err := m.UpdateFromSR(ssrc, baseRTP, hi, lo); err != nil {
		t.Fatal(err)
	}

	numPackets := 5
	// push 5 packets 10ms apart, total occupancy 40ms > 30ms target
	for i := 0; i < numPackets; i++ {
		p := Packet{
			Seq:   uint16(i),
			RtpTs: testutil.RTPAfter(clockRate, baseRTP, time.Duration(i)*10*time.Millisecond),
		}
		st, err := buf.Push(ssrc, p)

		if err != nil {
			t.Fatalf("Push error: %v", err)
		}
		if st != StatusAccepted {
			t.Fatalf("expected StatusAccepted, got %v", StatusString(st))
		}
	}

	jb := buf.(*jitterBuffer)
	jb.mu.Lock()
	sbuf, ok := jb.streams[ssrc]
	jb.mu.Unlock()
	if !ok {
		t.Fatal("expected ssrc buffer to be initialized")
	}

	sbuf.mu.Lock()
	defer sbuf.mu.Unlock()

	if sbuf.heap.Len() != numPackets-1 {
		t.Fatalf("expected heap size %d, got %d", numPackets-1, sbuf.heap.Len())
	}

	oldest := sbuf.heap.PeekPacket()
	if oldest == nil || oldest.Seq != 1 {
		t.Fatalf("expected oldest Seq 1, got %+v", oldest)
	}

	if sbuf.occupy > cfg.MaxTarget {
		t.Fatalf("expected occupancy <= %v, got %v", cfg.MaxTarget, sbuf.occupy)
	}
}
