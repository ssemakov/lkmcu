package clock

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
	baseRTP          uint64 // from SR NTP timestamp
	baseNTP          uint64 // from SR RTP timestamp
	offset           int64  // baseNtp - baseRtp/clockRate
	clockRate        uint32 // e.g., 90000 for video, 48000 for audio
	wrapTracker      RTPWrapTracker
	lastNTPOutput    uint64
	lastSRReceivedAt uint64
	baseSRRecvAt     uint64
	degraded         bool
}
