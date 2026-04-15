package media

import (
	"github.com/ssemakov/lkmcu/internal/media/decoders"
	"github.com/ssemakov/lkmcu/internal/media/depaketizers"
)

type CodecFactory interface {
	NewDepaketizer(mimeType string) (depaketizers.Depaketizer, error)
	NewDecoder(mimeType string) (decoders.Decoder, error)
}

type DefaultCodecFactory struct {
}

type ErrUnsupportedCodec struct {
}

func (e *ErrUnsupportedCodec) Error() string {
	return "unsupported codec"
}

func (f *DefaultCodecFactory) NewDepaketizer(mimeType string) (depaketizers.Depaketizer, error) {
	switch mimeType {
	case "audio/opus":
		return depaketizers.NewOpusDepaketizer(), nil
	default:
		return nil, &ErrUnsupportedCodec{}
	}
}

func (f *DefaultCodecFactory) NewDecoder(mimeType string) (decoders.Decoder, error) {
	switch mimeType {
	case "audio/opus":
		return decoders.NewOpusDecoder(), nil
	default:
		return nil, &ErrUnsupportedCodec{}
	}
}
