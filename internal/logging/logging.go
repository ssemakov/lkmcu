package logging

import "log"

type Logger struct{ region, level string }

func New(region, level string) *Logger              { return &Logger{region: region, level: level} }
func (l *Logger) prefix() string                    { return "[mcu-" + l.region + "] " }
func (l *Logger) Infof(f string, a ...interface{})  { log.Printf(l.prefix()+f, a...) }
func (l *Logger) Errorf(f string, a ...interface{}) { log.Printf(l.prefix()+"ERROR: "+f, a...) }
func (l *Logger) Fatalf(f string, a ...interface{}) { log.Fatalf(l.prefix()+f, a...) }
