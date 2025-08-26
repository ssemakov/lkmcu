package media

type Encoder interface {
  Open() error
  Close() error
  WriteVideoFrame([]byte) error
  WriteAudioFrame([]float32) error
  ReadRTP() ([]byte, error)
}
