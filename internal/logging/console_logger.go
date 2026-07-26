package logging

import "log"

type ConsoleLogger struct{ region, level string }

func NewConsoleLogger(region, level string) *ConsoleLogger {
	return &ConsoleLogger{region: region, level: level}
}
func (l *ConsoleLogger) prefix() string { return "[mcu-" + l.region + "]" }
func (l *ConsoleLogger) Tracef(f string, a ...interface{}) {
	if l.level == "trace" {
		log.Printf(l.prefix()+"[TRACE] "+f, a...)
	}
}
func (l *ConsoleLogger) Debugf(f string, a ...interface{}) {
	if l.level == "debug" || l.level == "trace" {
		log.Printf(l.prefix()+"[DEBUG] "+f, a...)
	}
}
func (l *ConsoleLogger) Infof(f string, a ...interface{})  { log.Printf(l.prefix()+"[INFO] "+f, a...) }
func (l *ConsoleLogger) Errorf(f string, a ...interface{}) { log.Printf(l.prefix()+"[ERROR] "+f, a...) }
func (l *ConsoleLogger) Fatalf(f string, a ...interface{}) { log.Fatalf(l.prefix()+"[FATAL] "+f, a...) }
