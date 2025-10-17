package clock

type RTPWrapTracker struct {
	lastRTP uint32
	wraps   uint32 // count of RTP wraps
}

type StreamClock struct {
	baseRTP          uint64
	baseNTP          uint64
	offset           int64
	wrapTracker      RTPWrapTracker
	lastNTPOutput    uint64
	lastSRReceivedAt uint64
	clockRate        uint32
	baseSRRecvAt     uint64
	degraded         bool
}
