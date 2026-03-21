package packet

import "github.com/pion/rtcp"

// Fields:
type SenderReport struct {
	RtpTs   uint32
	NtpHigh uint32
	NtpLow  uint32
}

// Payload must be read-only
func FromRtcpSR(src rtcp.SenderReport) SenderReport {
	return SenderReport{
		RtpTs:   src.RTPTime,
		NtpHigh: uint32(src.NTPTime >> 32),
		NtpLow:  uint32(src.NTPTime),
	}
}
