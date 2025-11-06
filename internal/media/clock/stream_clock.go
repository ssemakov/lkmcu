package clock

import (
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
}

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
