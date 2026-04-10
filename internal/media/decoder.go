package media

import "time"

type CodecID string

const (
	CodecOpus CodecID = "opus"
	CodecVP8  CodecID = "vp8"
	CodecH264 CodecID = "h264"
)

// Common timing/continuity metadata carried into decode.
type DecodeMeta struct {
	SSRC          uint32
	RTPTime       uint32
	PlayoutAt     time.Time // mapped / scheduled media time
	ArrivedAt     time.Time
	LostBefore    int  // number of missing RTP packets before this unit/frame
	Discontinuous bool // reset / seek / large gap / mapper jump
}

type Decoder interface {
	Open() error
	Close() error
	Reset() error
	Accept(EncodedUnit) error
}
