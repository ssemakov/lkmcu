package media

type Decoder interface {
  Open() error
  Close() error
  WriteRTP([]byte) error
  ReadVideoFrame() ([]byte, error)
  ReadAudioFrame() ([]float32, error)
}
