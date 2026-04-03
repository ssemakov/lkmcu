package clock

import (
	"math"
	"sync"
	"time"
)

// RTPWrapTracker tracks RTP timestamp wraparounds
// RTP timestamps are 32-bit and can wrap around
// at ~24.8 hours for a 48kHz clock rate, at ~13.3 hours for a 90kHz clock rate
// this allows us to extend RTP timestamps to 64-bit space
// For example
// Raw RTP timestamps:
//    0xFFFFFF00  →  0xFFFFFFA0  →  0x00000040  (wrap)
// Detected:
//    wrapTracker.wraps = 1
// Extended RTP timeline (64-bit):
//    0x00000000FFFFFF00
//    0x00000000FFFFFFA0
//    0x0000000100000040  ← corrected rollover

type RTPWrapTracker struct {
	lastRTP uint32
	wraps   uint32 // count of RTP wraps
}

type StreamClock struct {
	baseRTP          uint64  // from SR RTP timestamp
	baseNTP          float64 // from SR NTP timestamp; NTP as float64 seconds since epoch
	clockRate        uint32  // e.g., 90000 for video, 48000 for audio
	wrapTracker      RTPWrapTracker
	lastNTPOutput    float64   // monotonic guard, last NTP timestamp output, to detect clock jumps
	lastSRReceivedAt time.Time // monotonic time when last SR was received, don't convert to Unix to avoid clock jumps
	degraded         bool      // set when clock mapping is degraded due to lack of SRs (e.g., no SRs received for a while), or due to discontinuity (SR jumps more than threshold (50ms))
	discontinuity    bool      // set when last SR caused a discontinuity
	mu               sync.Mutex
	state            ClockState
}

type ClockState int

const (
	// No SR received yet, clock is not anchored to any RTP/NTP reference
	Uninitialized ClockState = iota
	// First SR received, clock is anchored to RTP/NTP reference, but not yet locked (e.g., not enough SRs received to confirm stability)
	Anchored
	// Clock is locked, SRs are stable, and mapping is reliable
	Locked
)

func (wt *RTPWrapTracker) ExtendedRTP(ts uint32) (ext uint64) {
	// Calculate extended RTP timestamp considering wraparounds
	// Detect wraparound for 32-bit RTP timestamp if rtp_ts < baseRTP and difference > 2^31
	if ts < wt.lastRTP && (wt.lastRTP-ts) > (1<<31) {
		wt.wraps++
	}
	wt.lastRTP = ts

	// extended monotonic RTP timestamp
	// extended RTP = rtp_ts + wraps * 2^32
	return uint64(ts) + uint64(wt.wraps)*(1<<32)
}

func (sc *StreamClock) Anchor(rtpTs uint32, ntpTsSec float64) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.state = Anchored
	sc.reset(rtpTs, ntpTsSec)
	sc.degraded = false
	sc.discontinuity = false
}

func (sc *StreamClock) Lock() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.state = Locked
}

func (sc *StreamClock) Update(rtpTs uint32, ntpTsSec float64, discontinuityThreshold time.Duration) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	extRtp := sc.wrapTracker.ExtendedRTP(rtpTs)
	discontinuity := sc.detectDiscontinuity(extRtp, ntpTsSec, discontinuityThreshold)

	if discontinuity {
		sc.setDiscontinuity(rtpTs, ntpTsSeconds)
	} else {
		sc.resetDiscontinuity()
		sc.baseNTP = ntpTsSeconds
		sc.baseRTP = extRtp
	}
}


func (sc *StreamClock) detectDiscontinuity(extRtp uint64, ntpTsSec float64, threshold time.Duration) bool {
	// Calculate expected NTP timestamp based on the anchored RTP/NTP reference
	predicted := sc.baseNTP + sc.deltaSeconds(extRtp)
	delta := ntpTsSec - predicted

	return math.Abs(delta) > threshold.Seconds()
}

func (sc *StreamClock) setDiscontinuity(ptsTs uint32, ntpTsSec float64) {) {
	sc.reset(ptsTs, ntpTsSec)
	sc.degraded = true
	sc.discontinuity = true
}

func (sc *StreamClock) resetDiscontinuity() {
	sc.degraded = false
	sc.discontinuity = false
}

// deltaSeconds calculates the time difference in seconds between the given extended RTP timestamp and the base RTP timestamp
func (sc *StreamClock) deltaSeconds(extRtp uint64) float64 {
	return (float64(extRtp) - float64(sc.baseRTP)) / float64(sc.clockRate)
}

func (sc *StreamClock) reset(rtpTs uint32, ntpTsSec float64) {
	sc.wrapTracker.wraps = 0
	sc.wrapTracker.lastRTP = rtpTs
	sc.baseRTP = uint64(rtpTs)
	sc.baseNTP = ntpTsSec
	sc.lastNTPOutput = ntpTsSec
	sc.lastSRReceivedAt = time.Now()
}
