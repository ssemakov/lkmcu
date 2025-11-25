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
	heap    *PacketHeap   // Min Heap to Push/Pop packets in desired order
	occupy  time.Duration // occupancy of the buffer in ms
	lastPop time.Time     // When did we last time Pop
	target  time.Duration // target buffer duration
	//burst   time.Duration
	freezeUntil time.Time                 // freeze cursor until time (according to config.RebufferBurst) to allow packets catchup
	recent      media.RingBuffer[dupeKey] // recent packets for duplicate detection, simple alternative to a full LRU cache

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
		DupeBufferSize:  100,
	}
}

func NewBuffer(mapper *clock.ClockMapper, cfg *Config) Buffer {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	if cfg.DupeBufferSize < cfg.MaxReorderDepth {
		panic("DupeBufferSize must be >= MaxReorderDepth")
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
		sb := SSRCBuffer{heap: &PacketHeap{}, mu: sync.Mutex{}, lastPop: time.Time{}, target: jb.cfg.Target,
			recent: *media.NewRingBuffer[dupeKey](jb.cfg.DupeBufferSize)}
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
	// TODO:
	// 1. late-drop threshold must adjust differently (more forgiving)
	// 2. Pop must become more conservative
	if state.Degraded {
		// if mapper degraded, use max target
		sbuf.target = jb.cfg.MaxTarget
		status = StatusMapperDegraded
	} else {
		sbuf.target = jb.cfg.Target
	}

	now := time.Now()
	pts := int64(packetNtp*1e9 + 0.5) // nano sec half-up rounded to int

	dupKey := dupeKey{pts: pts, seq: packet.Seq}
	if sbuf.recent.Has(dupKey) {
		return StatusDuplicate, nil
	} else {
		sbuf.recent.Push(dupKey)
	}

	// If heap empty for > underflow_guard_ms → pause cursor (rebuffer_burst_ms) and log underflow.
	if sbuf.heap.Len() == 0 && !sbuf.lastPop.IsZero() && now.Sub(sbuf.lastPop) > jb.cfg.UndeflowGuard {
		sbuf.freezeUntil = sbuf.freezeUntil.Add(-sbuf.target)
		return StatusRebuffering, nil
	}

	crs := sbuf.cursor(now)
	head := sbuf.heap.PeekPacket()
	packetPts := time.Unix(0, pts)

	// Drop late packets
	if packetPts.Before(crs.Add(-jb.cfg.LateDrop)) {
		return StatusLateDrop, nil
	}

	// first packet after underflow
	if head != nil {
		// drop oldest packets until occupancy ≤ target_ms
		for sbuf.occupy > sbuf.target && sbuf.heap.Len() > 0 {
			sbuf.updateOccupancy()
			sbuf.heap.PopPacket()
		}
	}

	packet.PayloadTs = packetPts
	sbuf.heap.PushPacket(&packet)
	sbuf.updateOccupancy()

	return status, nil
}

func (jb *jitterBuffer) Pop(ssrc uint32, now time.Time) (Packet, bool) {
	jb.mu.Lock()
	sbuf, ok := jb.streams[ssrc]
	jb.mu.Unlock()

	if !ok {
		return Packet{}, false
	}

	sbuf.mu.Lock()
	defer sbuf.mu.Unlock()

	p := sbuf.heap.PeekPacket()
	if p == nil {
		// If heap empty and cursor > last_packet_pts + guard_ms:
		// → underflow → burst, request keyframe (video)
		return Packet{}, false
	}

	// check if packet is ready to be popped based on current time and target buffer size
	crs := sbuf.cursor(now)
	ready := p.PayloadTs.Before(crs) || p.PayloadTs.Equal(crs)

	if !ready {
		return Packet{}, false
	}

	sbuf.lastPop = now
	p = sbuf.heap.PopPacket()
	sbuf.updateOccupancy()
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

func (sb *SSRCBuffer) cursor(now time.Time) time.Time {
	if now.Before(sb.freezeUntil) {
		// cursor stays frozen at the freeze start moment
		return sb.freezeUntil.Add(-sb.target)
	}

	return now.Add(-sb.target)
}
