package media

type Pipeline interface {
  Start() error
  Stop() error
}
