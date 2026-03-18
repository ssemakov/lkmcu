// Package jitter_buffer implements a simple, adaptive RTP jitter buffer
// suitable for audio/video playout. It smooths inter‑arrival jitter by
// admitting, ordering, and releasing packets against a playout cursor on
// a stable clock produced by the clock mapper.
//
// Design notes and expected behavior are described in docs/jitter_buffer.md.
package jitter_buffer

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/ssemakov/lkmcu/internal/logging"
	"github.com/ssemakov/lkmcu/internal/media/clock"
	"github.com/ssemakov/lkmcu/internal/media/common"
	"github.com/ssemakov/lkmcu/internal/media/packet"
)

type Status int

const (
	// StatusAccepted indicates the packet was admitted into the buffer.
	StatusAccepted Status = iota
	// StatusDuplicate indicates the packet matched a recently seen
	// {timestamp, sequence} pair and was ignored.
	StatusDuplicate
	// StatusLateDrop indicates the packet arrived too far behind the
	// playout cursor and was dropped (see Config.LateDrop).
	StatusLateDrop
	// StatusMapperDegraded indicates the packet was admitted but the
	// RTP→NTP clock mapper reported a degraded state; the buffer widens
	// to Config.MaxTarget for this SSRC while degraded. See docs/jitter_buffer.md.
	StatusMapperDegraded
	// StatusMapperError indicates RTP→NTP conversion failed; the packet
	// was not admitted and an error is returned.
	StatusMapperError
)

func StatusString(status Status) string {
	switch status {
	case StatusAccepted:
		return "StatusAccepted"
	case StatusDuplicate:
		return "StatusDuplicate"
	case StatusLateDrop:
		return "StatusLateDrop"
	case StatusMapperDegraded:
		return "StatusMapperDegraded"
	case StatusMapperError:
		return "StatusMapperError"
	default:
		return "UnknownStatus"
	}
}

// Buffer provides a concurrency‑safe jitter buffer interface keyed by SSRC.
//
// Contract (see docs/jitter_buffer.md):
//   - Push() admits packets, dropping late or duplicate packets.
//   - Pop() releases the next packet when its presentation timestamp is
//     ≤ the playout cursor (now - target).
//   - Occupancy() reports the time span between newest and oldest buffered
//     packets for the given SSRC.
//   - Target() reports the effective target depth for that SSRC; it
//     reflects mapper-driven widening. For unknown SSRCs, returns the
//     nominal configured target.
//
// Concurrency: safe for concurrent Push/Pop across SSRCs, and safe for
// concurrent Push/Pop on the same SSRC.
type Buffer interface {
	Push(ssrc uint32, packet packet.Packet) (Status, error)
	Pop(ssrc uint32, now time.Time) (packet.Packet, bool)
	Occupancy(ssrc uint32) time.Duration
	Target(ssrc uint32) time.Duration

	Config() *Config
}

// Config controls jitter buffer behavior. All durations are in time.Duration.
// Defaults follow the guidance in docs/jitter_buffer.md.
//
// Notes:
//   - LateDrop defines how far behind the playout cursor a packet may be
//     before it is dropped as too late.
//   - MaxTarget is the maximum buffer depth; when occupancy grows beyond
//     this value, oldest packets are dropped to cap latency.
//   - Target is the nominal buffer depth used in steady state; during clock
//     mapper degradation the effective per‑SSRC target widens to MaxTarget.
//   - RebufferBurst pauses the playout cursor by this amount when an
//     underflow is detected, allowing packets to catch up.
//   - UndeflowGuard is the duration an empty buffer must persist after
//     the last pop before underflow handling is triggered.
//   - MaxReorderDepth bounds tolerated reordering (in packets) to cap memory. (Memory Pressure gauge)
//   - DupeBufferSize must be ≥ MaxReorderDepth and limits the sliding window
//     used for duplicate detection.
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

// SSRCBuffer holds the per‑SSRC state: the timestamp‑ordered packet heap,
// target depth, and recent‑packet filter used for duplicate detection.
//
// This type is exposed for tests and debugging; external users should use
// the Buffer interface.
type SSRCBuffer struct {
	heap         *PacketHeap                // Min heap to Push/Pop packets in presentation order
	occupy       time.Duration              // Current occupancy (newest.PTS - oldest.PTS)
	lastPopPts   time.Time                  // Last popped packet presentation timestamp
	lastPopAt    time.Time                  // When the last pop occurred
	target       time.Duration              // Effective target buffer duration for this SSRC
	freezeUntil  time.Time                  // Cursor freeze deadline used during rebuffering
	freezeCursor time.Time                  // Cursor value while frozen
	recent       common.RingBuffer[dupeKey] // Sliding window used for duplicate detection
	noSR         bool                       // true if no Sender Report has been received yet for this SSRC

	mu sync.Mutex
}

// OnUnderflowHandler is invoked when an SSRC experiences buffer underflow.
// Typical handlers request a keyframe for video or log diagnostics.
type OnUnderflowHandler func(ssrc uint32)

// jitterBuffer implements the Buffer interface.
type jitterBuffer struct {
	streams     map[uint32]*SSRCBuffer // per-SSRC packet heaps
	mapper      *clock.ClockMapper
	cfg         *Config
	logger      logging.Logger
	onUnderflow OnUnderflowHandler

	mu sync.Mutex
}

// DefaultConfig returns a Config populated with conservative defaults that
// provide stable playout under typical jitter. See docs/jitter_buffer.md.
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

// NewBuffer creates a Buffer backed by a clock mapper and configuration.
// If cfg is nil, DefaultConfig is used. If onUnderflow is nil, a no‑op handler is installed.
// NewBuffer panics if DupeBufferSize < MaxReorderDepth.
func NewBuffer(mapper *clock.ClockMapper, logger logging.Logger, cfg *Config, onUnderflow OnUnderflowHandler) Buffer {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	if onUnderflow == nil {
		onUnderflow = func(_ uint32) {}
	}

	if cfg.DupeBufferSize < cfg.MaxReorderDepth {
		panic("DupeBufferSize must be >= MaxReorderDepth")
	}

	if cfg.MaxReorderDepth <= 0 {
		// TODO: consider disabling memory pressure checks if non-positive
		panic("JitterBuffer: MaxReorderDepth is non-positive")
	}

	return &jitterBuffer{
		streams:     make(map[uint32]*SSRCBuffer),
		mapper:      mapper,
		cfg:         cfg,
		onUnderflow: onUnderflow,
		logger:      logger,
		mu:          sync.Mutex{},
	}
}

// Push admits an RTP packet into the jitter buffer for the given SSRC.
// It converts RTP timestamp to a presentation timestamp using the mapper
// and enqueues the packet in timestamp order.
//
// Return values:
//   - StatusAccepted on success
//   - StatusDuplicate if a recent packet with the same {timestamp, sequence}
//     was seen and the packet is ignored
//   - StatusLateDrop if the packet arrived later than cursor - LateDrop
//   - StatusMapperDegraded if admitted while the mapper is degraded
//   - StatusMapperError with a non‑nil error if RTP→NTP mapping failed
func (jb *jitterBuffer) Push(ssrc uint32, packet packet.Packet) (Status, error) {
	jb.mu.Lock()

	sbuf, ok := jb.streams[ssrc]

	// Initialize per-SSRC packet heap if not present
	if !ok {
		sb := SSRCBuffer{heap: &PacketHeap{}, mu: sync.Mutex{}, target: jb.cfg.Target,
			recent: *common.NewRingBuffer[dupeKey](jb.cfg.DupeBufferSize), noSR: true}
		jb.streams[ssrc] = &sb
		sbuf = &sb
	}

	jb.mu.Unlock()

	sbuf.mu.Lock()
	defer sbuf.mu.Unlock()

	now := time.Now()
	pts := now.UnixNano() // default to now if no SR yet
	srAvailable := jb.mapper.HasSenderReport(ssrc)
	var packetNtp time.Time
	status := StatusAccepted
	mapperState := clock.MapperState{}
	var err error

	// Recalculate PTS for all buffered packets immediately upon receiving the first SR
	if sbuf.noSR && srAvailable {
		jb.logger.Infof("JitterBuffer: SSRC %08x received first SR; exiting no-SR mode", ssrc)
		// set noSR to false before recalculating PTS to enable proper mapper usage
		sbuf.noSR = false
		jb.recalculatePts(sbuf, ssrc)
	} else if !sbuf.noSR && !srAvailable {
		jb.logger.Infof("JitterBuffer: SSRC %08x lost SR; entering no-SR mode", ssrc)
		sbuf.noSR = true
	}

	// Normal operation when SR is available, map RTP→NTP
	// otherwise use now as PTS to allow buffering
	if srAvailable {
		packetNtp, mapperState, err = jb.mapper.RTPToNTPTime(ssrc, packet.RtpTs)
		pts = packetNtp.UnixNano()
	}

	// Handle mapper Discontinuity and Degraded states below, otherwise return error
	if !mapperState.Degraded && !mapperState.Discontinuity && err != nil {
		return StatusMapperError, err
	}

	// if state.Degraded buffer size is increased to max_target_ms
	// TODO:
	// 1. late-drop threshold must adjust differently (more forgiving)
	// 2. Pop must become more conservative
	if mapperState.Degraded {
		sbuf.target = jb.cfg.MaxTarget
		status = StatusMapperDegraded
	} else {
		sbuf.target = jb.cfg.Target
	}

	if sbuf.dupKey(pts, packet) {
		return StatusDuplicate, nil
	}

	unixPts := time.Unix(0, pts)

	// Drop late packets
	if r, e := sbuf.dropLate(unixPts, now, jb.cfg); !r {
		jb.logger.Debugf(e.Error())
		return StatusLateDrop, nil
	}

	tp := timedPacket{
		Packet:    packet,
		PayloadTs: unixPts,
	}

	sbuf.heap.PushPacket(&tp)
	sbuf.updateOccupancy()

	// Memory pressure check, drop overflowing the heap packets
	if r, e := sbuf.memoryPressure(ssrc, jb.cfg); !r {
		jb.logger.Errorf(e.Error())
	}

	// drop oldest
	sbuf.dropOldest()

	return status, nil
}

// Pop returns the next ready packet for the given SSRC at time now.
// A packet is ready when its presentation timestamp is ≤ the playout
// cursor (now - target). If the buffer is empty for longer than
// Config.UndeflowGuard after the last pop, the cursor is temporarily
// frozen by Config.RebufferBurst and onUnderflow is invoked.
func (jb *jitterBuffer) Pop(ssrc uint32, now time.Time) (packet.Packet, bool) {
	jb.mu.Lock()
	sbuf, ok := jb.streams[ssrc]
	jb.mu.Unlock()

	if !ok {
		return packet.Packet{}, false
	}

	sbuf.mu.Lock()
	defer sbuf.mu.Unlock()

	if sbuf.noSR {
		return packet.Packet{}, false
	}

	tp := sbuf.heap.PeekPacket()

	if tp == nil && sbuf.lastPopPts.IsZero() {
		// buffer is empty, and we have never popped any packet yet
		return packet.Packet{}, false
	} else if tp == nil {
		// buffer is empty, check for underflow condition
		deadline := sbuf.lastPopAt.Add(jb.cfg.UndeflowGuard)
		if now.After(deadline) {
			// underflow, freeze cursor
			if now.After(sbuf.freezeUntil) {
				sbuf.freezeUntil = now.Add(jb.cfg.RebufferBurst)
				sbuf.freezeCursor = now.Add(-sbuf.target)

				// request key frame or whatever else decoder wants to do
				// once per underflow event
				jb.onUnderflow(ssrc)
			}
		}
		return packet.Packet{}, false
	}

	cursor := sbuf.cursor(now)
	// check if packet is ready to be popped based on current time and target buffer size
	ready := tp.PayloadTs.Before(cursor) || tp.PayloadTs.Equal(cursor)

	if !ready {
		return packet.Packet{}, false
	}

	tp = sbuf.heap.PopPacket()
	sbuf.lastPopPts = tp.PayloadTs
	sbuf.lastPopAt = now
	sbuf.updateOccupancy()
	return tp.Packet, true
}

// Occupancy reports the current time span between the newest and oldest
// packets in the buffer for the given SSRC.
func (jb *jitterBuffer) Occupancy(ssrc uint32) time.Duration {
	jb.mu.Lock()
	sbuf, ok := jb.streams[ssrc]
	jb.mu.Unlock()

	if ok {
		sbuf.mu.Lock()
		defer sbuf.mu.Unlock()

		return sbuf.occupy
	}

	return 0
}

// Target returns the effective target buffer depth for the given SSRC.
// If the SSRC is unknown, it returns the nominal configured target.
func (jb *jitterBuffer) Target(ssrc uint32) time.Duration {
	jb.mu.Lock()
	sbuf, ok := jb.streams[ssrc]
	jb.mu.Unlock()

	if ok {
		sbuf.mu.Lock()
		defer sbuf.mu.Unlock()

		return sbuf.target
	}

	return jb.cfg.Target
}

func (jb *jitterBuffer) Config() *Config {
	return jb.cfg
}

func (jb *jitterBuffer) recalculatePts(sbuf *SSRCBuffer, ssrc uint32) {
	// Thread safety is granted by the caller (caller holds sbuf.mu)
	packets := make([]*timedPacket, 0, sbuf.heap.Len())
	for sbuf.heap.Len() > 0 {
		packets = append(packets, sbuf.heap.PopPacket())
	}

	// Sort packets by RTP timestamp and sequence number to avoid out of order PTS
	// (This is different from the normal payload order used in the heap by Less() that uses PTS first)
	// and triggering mapper's monotonic guard
	sort.Slice(packets, func(i, j int) bool {
		if packets[i].RtpTs == packets[j].RtpTs {
			return packets[i].Seq < packets[j].Seq
		}
		return packets[i].RtpTs < packets[j].RtpTs
	})

	nHeap := &PacketHeap{}

	// Clear recent dupeKey buffer as PTS are changing
	sbuf.recent.Clear()

	for _, tp := range packets {
		packetNtp, _, err := jb.mapper.RTPToNTPTime(ssrc, tp.RtpTs)
		if err != nil {
			jb.logger.Debugf("Recalculate Pts: failed to remap RTP ts %d for SSRC %08x: %v",
				tp.RtpTs, ssrc, err)
			continue
		}

		if sbuf.dupKey(packetNtp.UnixNano(), tp.Packet) {
			continue
		}

		unixPts := time.Unix(0, packetNtp.UnixNano())
		now := time.Now()

		if r, e := sbuf.dropLate(unixPts, now, jb.cfg); !r {
			jb.logger.Debugf("Recalculate Pts: dropping late packet RTP ts %d for SSRC %08x: %v",
				tp.RtpTs, ssrc, e)
			continue
		}

		tp.PayloadTs = unixPts
		nHeap.PushPacket(tp)
	}

	sbuf.heap = nHeap
	sbuf.updateOccupancy()

	// Memory pressure check, drop overflowing the heap packets
	if r, e := sbuf.memoryPressure(ssrc, jb.cfg); !r {
		jb.logger.Errorf(e.Error())
	}

	// drop oldest
	sbuf.dropOldest()
}

func (sbuf *SSRCBuffer) dupKey(pts int64, packet packet.Packet) bool {
	// Thread safety is granted by the caller (caller holds sb.mu)

	dupKey := dupeKey{pts: pts, seq: packet.Seq}
	if sbuf.recent.Has(dupKey) {
		return true
	} else {
		sbuf.recent.Push(dupKey)
	}

	return false
}

func (sbuf *SSRCBuffer) dropLate(packetPts time.Time, now time.Time, cfg *Config) (bool, error) {
	// Thread safety is granted by the caller (caller holds sb.mu)

	cursor := sbuf.cursor(now)

	if !sbuf.noSR && packetPts.Before(cursor.Add(-cfg.LateDrop)) {
		return false, fmt.Errorf("LateDrop: packetPts=%v before cursor=%v (target=%v) - Config.LateDrop=%v",
			packetPts, cursor, cfg.Target, cfg.LateDrop)
	}

	return true, nil
}

func (sbuf *SSRCBuffer) memoryPressure(ssrc uint32, cfg *Config) (bool, error) {
	// Thread safety is granted by the caller (caller holds sb.mu)

	overflow := sbuf.heap.Len() - cfg.MaxReorderDepth

	if overflow > 0 {
		before := sbuf.heap.Len()
		for i := 0; i < overflow; i++ {
			sbuf.heap.PopPacket()
		}
		sbuf.updateOccupancy()

		return false, fmt.Errorf("JitterBuffer: max_reorder_depth=%d exceeded by %d; dropped oldest for SSRC %08x (len %d→%d, target=%v, occupancy=%v)",
			cfg.MaxReorderDepth, overflow, ssrc, before, sbuf.heap.Len(), sbuf.target, sbuf.occupy)
	}

	return true, nil
}

// drop oldest packets until occupancy ≤ target_ms
func (sbuf *SSRCBuffer) dropOldest() {
	for sbuf.heap.Len() > 0 {
		sbuf.updateOccupancy()
		if sbuf.occupy <= sbuf.target {
			return
		}
		sbuf.heap.PopPacket()
	}
}

func (sbuf *SSRCBuffer) updateOccupancy() {
	oldest := sbuf.heap.PeekPacket()
	newest := sbuf.heap.PeekMaxPacket()

	if oldest != nil && newest != nil {
		sbuf.occupy = newest.PayloadTs.Sub(oldest.PayloadTs)
	} else {
		sbuf.occupy = 0
	}
}

func (sbuf *SSRCBuffer) cursor(now time.Time) time.Time {
	if now.Before(sbuf.freezeUntil) {
		// cursor stays frozen at the freeze start moment
		return sbuf.freezeCursor
	}

	return now.Add(-sbuf.target)
}
