package jitter_buffer

import (
	"sync"
	"time"

	"github.com/ssemakov/lkmcu/internal/media"
	"github.com/ssemakov/lkmcu/internal/media/clock"
)

type Status int

const (
	StatusAccepted Status = iota
	StatusDuplicate
	StatusLateDrop
	StatusMapperDegraded
	StatusMapperError
	StatusRebuffering
)

type PacketPayload []byte

type Packet struct {
	RtpTs     uint32
	PayloadTs time.Time
	Seq       uint16
	Payload   PacketPayload
}

type Buffer interface {
	Push(ssrc uint32, packet Packet, mapperState clock.MapperState) (Status, error)
	Pop(ssrc uint32, now time.Time) (Packet, bool)
	Occupancy(ssrc uint32) time.Duration
	Target() time.Duration
}

type Config struct {
	LateDrop        time.Duration // packets older than target_ms + late_drop_ms are dropped
	MaxTarget       time.Duration // when Occupancy > max_target_ms start dropping oldest packets
	Target          time.Duration // desired target buffer size in ms, if mapper is degraded, buffer size is increased to max_target_ms
	RebufferBurst   time.Duration // when buffer is empty for more than underflow_guard_ms, pause cursor by rebuffer_burst_ms
	UndeflowGuard   time.Duration // duration of empty buffer to consider it underflowed (not having enough packets to pop)
	MaxReorderDepth int           // maximum expected packet reordering depth in packets
	DupeBufferSize  int           // number of recent packets to track for duplicate detection
}

type dupeKey struct {
	pts int64  // packet timestamp in nanos
	seq uint16 // packet sequence number
}

type SSRCBuffer struct {
	heap    *PacketHeap
	occupy  time.Duration
	lastPop time.Time
	target  time.Duration
	burst   time.Duration
	recent  media.RingBuffer[dupeKey] // recent packets for duplicate detection, simple alternative to a full LRU cache

	mu sync.Mutex
}

type jitterBuffer struct {
	streams map[uint32]*SSRCBuffer // per-SSRC packet heaps
	mapper  *clock.ClockMapper
	cfg     *Config

	mu sync.Mutex
}

func DefaultConfig() *Config {
	return &Config{
		LateDrop:        20 * time.Millisecond,
		MaxTarget:       200 * time.Millisecond,
		Target:          80 * time.Millisecond,
		RebufferBurst:   20 * time.Millisecond,
		UndeflowGuard:   20 * time.Millisecond,
		MaxReorderDepth: 50,
	}
}

func NewBuffer(mapper *clock.ClockMapper, cfg *Config) Buffer {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	return &jitterBuffer{
		streams: make(map[uint32]*SSRCBuffer),
		mapper:  mapper,
		cfg:     cfg,
		mu:      sync.Mutex{},
	}
}

func (jb *jitterBuffer) Push(ssrc uint32, packet Packet, mapperState clock.MapperState) (Status, error) {
	jb.mu.Lock()

	sbuf, ok := jb.streams[ssrc]

	// Initialize per-SSRC packet heap if not present
	if !ok {
		sb := SSRCBuffer{heap: &PacketHeap{}, mu: sync.Mutex{}, occupy: 0, lastPop: time.Time{}, target: jb.cfg.Target,
			burst: 0, recent: *media.NewRingBuffer[dupeKey](jb.cfg.DupeBufferSize)}
		jb.streams[ssrc] = &sb
		sbuf = &sb
	}

	jb.mu.Unlock()

	sbuf.mu.Lock()
	defer sbuf.mu.Unlock()
	packetNtp, state, err := jb.mapper.RTPToNTP(ssrc, packet.RtpTs)

	if err != nil {
		return StatusMapperError, err
	}

	status := StatusAccepted

	// if state.Degraded buffer size is increased to max_target_ms
	if state.Degraded {
		// if mapper degraded, use max target
		sbuf.target = jb.cfg.MaxTarget
		status = StatusMapperDegraded
	} else {
		sbuf.target = jb.cfg.Target
	}

	now := time.Now()
	cursor := now.Add(-sbuf.target).Add(sbuf.burst)
	head := sbuf.heap.PeekPacket()
	pts := int64(packetNtp*1e9 + 0.5) // nano sec half-up rounded to int
	packetPts := time.Unix(0, pts)

	dupKey := dupeKey{pts: pts, seq: packet.Seq}
	if sbuf.recent.Has(dupKey) {
		status = StatusDuplicate
		return status, nil
	} else {
		sbuf.recent.Push(dupKey)
	}

	// first packet after underflow
	if head == nil {
		sbuf.occupy = 0
	} else {
		// If heap empty for > underflow_guard_ms → pause cursor (rebuffer_burst_ms) and log underflow.
		if sbuf.heap.Len() == 0 && !sbuf.lastPop.IsZero() && now.Sub(sbuf.lastPop) > jb.cfg.UndeflowGuard {
			sbuf.burst = jb.cfg.RebufferBurst
			status = StatusRebuffering
		} else {
			sbuf.burst = 0
		}

		if packetPts.Before(cursor.Add(-jb.cfg.LateDrop)) {
			// return immediately without adding packet to heap
			return StatusLateDrop, nil
		}

		// drop oldest packets until occupancy ≤ target_ms
		for sbuf.occupy > sbuf.target && sbuf.heap.Len() > 0 {
			sbuf.heap.PopPacket()
			sbuf.updateOccupancy()
		}
	}

	packet.PayloadTs = packetPts
	sbuf.heap.PushPacket(&packet)
	sbuf.updateOccupancy()

	return status, nil
}

func (jb *jitterBuffer) Pop(ssrc uint32, now time.Time) (Packet, bool) {
	jb.mu.Lock()
	s, ok := jb.streams[ssrc]
	jb.mu.Unlock()

	if !ok {
		return Packet{}, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	p := s.heap.PeekPacket()
	if p == nil {
		return Packet{}, false
	}

	// check if packet is ready to be popped based on current time and target buffer size
	// packet_pts + target_ms ≤ now
	cursor := now.Add(-s.target).Add(-s.burst)
	ready := p.PayloadTs.Before(cursor) || p.PayloadTs.Equal(cursor)

	if !ready {
		return Packet{}, false
	}

	s.lastPop = now
	p = s.heap.PopPacket()
	s.updateOccupancy()
	return *p, true
}

func (jb *jitterBuffer) Occupancy(ssrc uint32) time.Duration {
	jb.mu.Lock()
	defer jb.mu.Unlock()

	return jb.streams[ssrc].occupy
}

func (jb *jitterBuffer) Target() time.Duration {
	jb.mu.Lock()
	defer jb.mu.Unlock()

	return jb.cfg.Target
}

func (sb *SSRCBuffer) updateOccupancy() {
	oldest := sb.heap.PeekPacket()
	newest := sb.heap.PeekMaxPacket()

	if oldest != nil && newest != nil {
		sb.occupy = newest.PayloadTs.Sub(oldest.PayloadTs)
	} else {
		sb.occupy = 0
	}
}
