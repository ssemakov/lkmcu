package testutil

import "math"

func ToNTPParts(ntp float64) (uint32, uint32) {
	sec, frac := math.Modf(ntp)
	// Convert fractional seconds to 32-bit NTP fractional field with rounding
	fl := math.Round(frac * (1 << 32)) // 2^32
	if fl >= (1 << 32) {
		sec += 1
		return uint32(sec), 0
	}
	return uint32(sec), uint32(fl)
}
