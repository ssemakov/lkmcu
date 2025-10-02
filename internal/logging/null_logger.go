package logging

type NullLogger struct{}

func NewNullLogger(region, level string) *NullLogger    { return &NullLogger{} }
func (l *NullLogger) Debugf(f string, a ...interface{}) {}
func (l *NullLogger) Infof(f string, a ...interface{})  {}
func (l *NullLogger) Errorf(f string, a ...interface{}) {}
func (l *NullLogger) Fatalf(f string, a ...interface{}) {}
