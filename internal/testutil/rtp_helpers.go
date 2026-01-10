package testutil

import (
	"math"
	"time"
)

const ntpEpochOffsetSeconds = 2208988800 // 1900-01-01 to 1970-01-01

// converts a Unix time to NTP seconds.
func UnixToNTPSeconds(t time.Time) float64 {
	return float64(t.UnixNano())/1e9 + ntpEpochOffsetSeconds
}

// returns a realistic SR anchor close to now and the NTP parts.
// - anchorAgo defines how far before now to place the SR (use ~80–150 ms).
// - Returns baseRTP (0), hi/lo (NTP parts), baseNTP (float), baseUnix.
func MakeSRBase(clockRate uint32, anchorAgo time.Duration) (baseRTP uint32, hi, lo uint32, baseNTP float64,
	baseUnix time.Time) {

	if anchorAgo <= 0 {
		anchorAgo = 100 * time.Millisecond
	}

	baseUnix = time.Now().Add(-anchorAgo)
	baseNTP = UnixToNTPSeconds(baseUnix)
	baseRTP = 0
	hi, lo = ToNTPParts(baseNTP)

	return
}

// returns the RTP timestamp offset by d since baseRTP at clockRate.
func RTPAfter(clockRate uint32, baseRTP uint32, d time.Duration) uint32 {
	if d <= 0 {
		return baseRTP
	}

	incr := uint32(math.Round(float64(clockRate) * d.Seconds()))
	return baseRTP + incr
}

// returns a consistent second SR at delta after base.
// nextRTP = baseRTP + clockRate*delta;
// nextHi/nextLo = NTP(baseNTP + delta).
func MakeNextSR(baseRTP uint32, baseNTP float64, clockRate uint32, delta time.Duration) (nextRTP uint32,
	nextHi, nextLo uint32, nextNTP float64) {

	if delta < 0 {
		delta = 0
	}

	nextRTP = baseRTP + uint32(math.Round(float64(clockRate)*delta.Seconds()))
	nextNTP = baseNTP + delta.Seconds()
	nextHi, nextLo = ToNTPParts(nextNTP)

	return
}
