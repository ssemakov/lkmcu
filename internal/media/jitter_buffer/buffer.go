package jitter_buffer

import (
	"container/heap"
	"sync"
	"time"

	"github.com/ssemakov/lkmcu/internal/media/clock"
)

type Status int

const (
	StatusAccepted Status = iota
	StatusDuplicate
	StatusLateDrop
	StatusMapperDegraded
	StatusMapperError
)

type PacketPayload []byte

type Packet struct {
	RtpTs     uint32
	PayloadTs time.Time
	Seq       uint16
	Payload   *PacketPayload
}

type Buffer interface {
	Push(ssrc uint32, packet Packet, mapperState clock.MapperState) (Status, error)
	Pop(ssrc uint32, now time.Time) (Packet, bool)
	Occupancy(ssrc uint32) time.Duration
	Target() time.Duration
}

type Config struct {
	LateDrop  time.Duration // packets older than target_ms + late_drop_ms are dropped
	MaxTarget time.Duration // when Occupancy > max_target_ms start dropping oldest packets
	Target    time.Duration // desired target buffer size in ms, if mapper is degraded, buffer size is increased to max_target_ms
}

type SSRCBuffer struct {
	heap    *PacketHeap
	occupy  time.Duration
	lastPop time.Time
	target  time.Duration

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
		LateDrop:  20 * time.Millisecond,
		MaxTarget: 200 * time.Millisecond,
		Target:    80 * time.Millisecond,
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

	s, ok := jb.streams[ssrc]

	// Initialize per-SSRC packet heap if not present
	if !ok {
		sb := SSRCBuffer{heap: &PacketHeap{}, mu: sync.Mutex{}, occupy: 0, lastPop: time.Time{}}
		jb.streams[ssrc] = &sb
		s = &sb
	}

	jb.mu.Unlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	ntp, state, err := jb.mapper.RTPToNTP(ssrc, packet.RtpTs)

	if err != nil {
		return StatusMapperError, err
	}

	// if state.Degraded buffer size is increased to max_target_ms
	if state.Degraded {
		// if mapper degraded, use max target
		s.target = jb.cfg.MaxTarget
	} else {
		s.target = jb.cfg.Target
	}

	// 	When to drop / underflow
	now := time.Now()
	cursor := now.Add(-s.target)
	if s.heap.Peek().PayloadTs.Before(cursor.Add(-jb.cfg.LateDrop)) {
		return StatusLateDrop, nil
	}

	if jb.Occupancy(ssrc) > s.target {
		// drop oldest packet
		oldest := heap.Pop(s.heap).(*Packet)
		newest := s.heap.Peek()
		if newest != nil {
			s.occupy = newest.PayloadTs.Sub(oldest.PayloadTs)
		}
	}
	// If heap empty for > underflow_guard_ms → pause cursor (rebuffer_burst_ms) and log underflow.

	packet.PayloadTs = time.Unix(0, int64(ntp*1e9))
	heap.Push(s.heap, &packet)
	s.occupy = now.Sub(s.heap.Peek().PayloadTs)

	return StatusAccepted, nil
}

func (jb *jitterBuffer) Pop(ssrc uint32, now time.Time) (Packet, bool) {
	jb.mu.Lock()
	s, ok := jb.streams[ssrc]
	jb.mu.Unlock()

	if !ok {
		return Packet{}, false
	}

	p := s.heap.Peek()
	if p != nil {
		// check if packet is ready to be popped based on current time and target buffer size
		// packet_pts + target_ms ≤ now
		cursor := now.Add(-s.target)
		ready := p.PayloadTs.Before(cursor) || p.PayloadTs.Equal(cursor)
		if ready {
			s.mu.Lock()
			defer s.mu.Unlock()
			s.occupy -= now.Sub(p.PayloadTs)
			s.lastPop = now
			p = heap.Pop(s.heap).(*Packet)
			return *p, true
		}

		return Packet{}, false
	}

	// in consumer routine:
	// for {
	//     pkt, ok := jb.buf.Pop(ssrc, time.Now())
	//     if ok {
	//         out <- pkt
	//     } else {
	//         time.Sleep(time.Millisecond)
	//     }
	// }

	return Packet{}, false
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
