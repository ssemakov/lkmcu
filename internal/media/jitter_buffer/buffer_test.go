package jitter_buffer

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/ssemakov/lkmcu/internal/logging"
	"github.com/ssemakov/lkmcu/internal/media/clock"
	mediaPacket "github.com/ssemakov/lkmcu/internal/media/packet"
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

	packet := mediaPacket.Packet{Seq: 1}

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

	baseRTP, hi, lo, baseNTP, _ := testutil.MakeSRBase(clockRate, 50*time.Millisecond)

	if err := m.UpdateFromSR(ssrc, baseRTP, hi, lo); err != nil {
		t.Fatal(err)
	}

	p1RTP := testutil.RTPAfter(clockRate, baseRTP, 50*time.Millisecond)
	p1 := mediaPacket.Packet{Seq: 1, RtpTs: p1RTP}

	status, err := buf.Push(ssrc, p1)

	if err != nil {
		t.Fatalf("no error expected, got %v", err)
	}

	if status != StatusAccepted {
		t.Fatalf("want StatusAccepted, got %v", StatusString(status))
	}

	baseRTP, hi, lo, _ = testutil.MakeNextSR(baseRTP, baseNTP, clockRate, 1*time.Second)
	if err := m.UpdateFromSR(ssrc, baseRTP, hi, lo); err != nil {
		t.Fatal(err)
	}

	p2RTP := testutil.RTPAfter(clockRate, baseRTP, 50*time.Millisecond)
	p2 := mediaPacket.Packet{Seq: 2, RtpTs: p2RTP}

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
	p1 := mediaPacket.Packet{Seq: 1, RtpTs: p1RTP}

	_, err := buf.Push(ssrc, p1)

	if err != nil {
		t.Fatalf("no error expected, got %v", err)
	}

	p2 := mediaPacket.Packet{Seq: 1, RtpTs: p1RTP}
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
	p1 := mediaPacket.Packet{Seq: 1, RtpTs: p1RTP}

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
	logger := logging.NewConsoleLogger("test", "debug")
	mCfg := clock.DefaultClockMapperConfig()
	mCfg.SRTimeout = 1 * time.Millisecond
	m := clock.NewClockMapper(metrics.NewMetrics(), logger, mCfg)
	buf := NewBuffer(m, logger, nil, nil)
	m.SetClockRate(ssrc, clockRate)

	baseRTP, hi, lo, _, _ := testutil.MakeSRBase(clockRate, 50*time.Millisecond)

	if err := m.UpdateFromSR(ssrc, baseRTP, hi, lo); err != nil {
		t.Fatal(err)
	}

	time.Sleep(5 * time.Millisecond)

	p1RTP := testutil.RTPAfter(clockRate, baseRTP, 50*time.Millisecond)
	p1 := mediaPacket.Packet{Seq: 1, RtpTs: p1RTP}

	status, err := buf.Push(ssrc, p1)

	if err != nil {
		t.Fatalf("no error expected, got %v", err)
	}

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

	baseRTP, hi, lo, _, _ := testutil.MakeSRBase(clockRate, 50*time.Millisecond)

	preSR := []mediaPacket.Packet{
		{Seq: 1, RtpTs: testutil.RTPAfter(clockRate, baseRTP, 10*time.Millisecond)},
		{Seq: 2, RtpTs: testutil.RTPAfter(clockRate, baseRTP, 20*time.Millisecond)},
	}
	for _, p := range preSR {
		if _, err := buf.Push(ssrc, p); err != nil {
			t.Fatalf("pre-SR push failed: %v", err)
		}
	}

	// Pop should not return any packets before SR
	pkt, ok := buf.Pop(ssrc, time.Now())
	if ok {
		t.Fatalf("expected no packet before SR, got %+v", pkt)
	}

	if err := m.UpdateFromSR(ssrc, baseRTP, hi, lo); err != nil {
		t.Fatal(err)
	}

	postSR := mediaPacket.Packet{Seq: 99, RtpTs: testutil.RTPAfter(clockRate, baseRTP, 30*time.Millisecond)}
	if _, err := buf.Push(ssrc, postSR); err != nil {
		t.Fatalf("post-SR push failed: %v", err)
	}

	pkt, ok = buf.Pop(ssrc, time.Now().Add(150*time.Millisecond))
	if !ok {
		t.Fatal("expected post-SR packet to pop")
	}
	if pkt.Seq != postSR.Seq {
		t.Fatalf("expected packet with Seq %d, got %d", postSR.Seq, pkt.Seq)
	}

	pkt, ok = buf.Pop(ssrc, time.Now().Add(150*time.Millisecond))
	if ok {
		t.Fatalf("expected pre-SR packets to be flushed, got %+v", pkt)
	}
}

func TestBuffer_Pop_HoldsPacketsUntilSRArrives(t *testing.T) {
	const (
		ssrc      = uint32(0xdeadbeef)
		clockRate = uint32(90000)
	)
	buf, m := newTestBuffer(nil, nil)
	m.SetClockRate(ssrc, clockRate)

	baseRTP, hi, lo, _, _ := testutil.MakeSRBase(clockRate, 50*time.Millisecond)

	// Push packets before any SR
	p1 := mediaPacket.Packet{Seq: 1, RtpTs: testutil.RTPAfter(clockRate, baseRTP, 15*time.Millisecond)}
	buf.Push(ssrc, p1)

	pkt, ok := buf.Pop(ssrc, time.Now())
	if ok {
		t.Fatalf("expected no packet before SR, got %+v", pkt)
	}

	// Now update SR
	if err := m.UpdateFromSR(ssrc, baseRTP, hi, lo); err != nil {
		t.Fatal(err)
	}

	// Push another packet after SR. The pre-SR packet should have been flushed.
	p2 := mediaPacket.Packet{Seq: 2, RtpTs: testutil.RTPAfter(clockRate, baseRTP, 20*time.Millisecond)}
	buf.Push(ssrc, p2)

	now := time.Now().Add(120 * time.Millisecond)
	pkt, ok = buf.Pop(ssrc, now)
	if !ok {
		t.Fatalf("expected packet after SR, got none")
	}

	if pkt.Seq != 2 {
		t.Fatalf("expected packet with Seq 2, got %d", pkt.Seq)
	}

	pkt, ok = buf.Pop(ssrc, now)
	if ok {
		t.Fatalf("expected pre-SR packet to be flushed, got %+v", pkt)
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
	p1 := mediaPacket.Packet{Seq: 1, RtpTs: testutil.RTPAfter(clockRate, baseRTP, 10*time.Millisecond)}
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

	baseRTP, hi, lo, _, _ := testutil.MakeSRBase(clockRate, 50*time.Millisecond)

	if err := m.UpdateFromSR(ssrc, baseRTP, hi, lo); err != nil {
		t.Fatal(err)
	}

	// Push packets
	numPackets := 5
	for i := 0; i < numPackets; i++ {
		p := mediaPacket.Packet{Seq: uint16(i), RtpTs: testutil.RTPAfter(clockRate, baseRTP, time.Duration(i)*10*time.Millisecond)}
		buf.Push(ssrc, p)
	}

	occ := buf.Occupancy(ssrc)
	expectedOcc := time.Duration(numPackets-1) * 10 * time.Millisecond

	if absDuration := occ - expectedOcc; absDuration < 0 {
		absDuration = -absDuration
		if absDuration > 2*time.Millisecond {
			t.Fatalf("expected occupancy %d, got %d", expectedOcc, occ)
		}
	} else {
		if absDuration > 2*time.Millisecond {
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

	baseRTP, hi, lo, baseNTP, _ := testutil.MakeSRBase(clockRate, 50*time.Millisecond)

	if err := m.UpdateFromSR(ssrc, baseRTP, hi, lo); err != nil {
		t.Fatal(err)
	}

	nom := buf.Target(0xabadcafe) // unknown SSRC

	if nom != buf.Config().Target {
		t.Fatalf("want nominal Target %v, got %v", buf.Config().Target, nom)
	}

	p := mediaPacket.Packet{Seq: 1, RtpTs: testutil.RTPAfter(clockRate, baseRTP, 10*time.Millisecond)}
	buf.Push(ssrc, p)

	if buf.Target(ssrc) != buf.Config().Target {
		t.Fatalf("want per-SSRC Target %v, got %v", buf.Config().Target, buf.Target(ssrc))
	}

	baseRTP, hi, lo, _ = testutil.MakeNextSR(baseRTP, baseNTP, clockRate, 10*time.Microsecond)
	if err := m.UpdateFromSR(ssrc, baseRTP, hi, lo); err != nil {
		t.Fatal(err)
	}

	p = mediaPacket.Packet{Seq: 2, RtpTs: testutil.RTPAfter(clockRate, baseRTP, 20*time.Millisecond)}
	buf.Push(ssrc, p)

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

	sb.heap.PushPacket(&timedPacket{Packet: mediaPacket.Packet{Seq: 2}, PayloadTs: base.Add(10 * time.Millisecond)})
	sb.updateOccupancy()
	if sb.occupy != 0 {
		t.Fatalf("expected occupancy 0 for single packet, got %v", sb.occupy)
	}

	sb.heap.PushPacket(&timedPacket{Packet: mediaPacket.Packet{Seq: 10}, PayloadTs: base.Add(0)})
	sb.heap.PushPacket(&timedPacket{Packet: mediaPacket.Packet{Seq: 1}, PayloadTs: base.Add(20 * time.Millisecond)})
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
		p := mediaPacket.Packet{
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

func TestBuffer_Push_MaxReorderDepthCap(t *testing.T) {
	const ssrc = uint32(0xdeadbeef)

	cfg := DefaultConfig()
	cfg.MaxReorderDepth = 3
	cfg.Target = time.Second
	cfg.MaxTarget = cfg.Target

	buf, _ := newTestBuffer(cfg, nil)

	// Expect 2 packets to be dropped when pushing total of MaxReorderDepth + 2 packets
	total := cfg.MaxReorderDepth + 2
	for i := 0; i < total; i++ {
		if _, err := buf.Push(ssrc, mediaPacket.Packet{Seq: uint16(i)}); err != nil {
			t.Fatalf("Push %d returned error: %v", i, err)
		}
	}

	jb := buf.(*jitterBuffer)
	jb.mu.Lock()
	sbuf, ok := jb.streams[ssrc]
	jb.mu.Unlock()
	if !ok {
		t.Fatal("expected SSRC buffer to exist")
	}

	sbuf.mu.Lock()
	defer sbuf.mu.Unlock()

	if got := sbuf.heap.Len(); got != cfg.MaxReorderDepth {
		t.Fatalf("expected heap length %d, got %d", cfg.MaxReorderDepth, got)
	}

	oldest := sbuf.heap.PeekPacket()
	expectedSeq := uint16(total - cfg.MaxReorderDepth)
	if oldest == nil || oldest.Seq != expectedSeq {
		t.Fatalf("expected oldest seq %d after trimming, got %+v", expectedSeq, oldest)
	}
}
