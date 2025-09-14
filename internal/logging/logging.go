package logging

import (
	"fmt"
	"log"
	"reflect"
)

type Logger struct{ region, level string }

// Formatter interface for types that can format themselves for logging
type Formatter interface {
	Format() string
}

func New(region, level string) *Logger              { return &Logger{region: region, level: level} }
func (l *Logger) prefix() string                    { return "[mcu-" + l.region + "] " }
func (l *Logger) Debugf(f string, a ...interface{}) { if l.level == "debug" { log.Printf(l.prefix()+f, a...) } }
func (l *Logger) Infof(f string, a ...interface{})  { log.Printf(l.prefix()+f, a...) }
func (l *Logger) Errorf(f string, a ...interface{}) { log.Printf(l.prefix()+"ERROR: "+f, a...) }
func (l *Logger) Fatalf(f string, a ...interface{}) { log.Fatalf(l.prefix()+f, a...) }

// FormatValue uses Formatter interface if available, otherwise returns default format
func FormatValue[T any](v T) string {
	if formatter, ok := any(v).(Formatter); ok {
		return formatter.Format()
	}

	val := reflect.ValueOf(v)
	typ := reflect.TypeOf(v)

	if typ.Kind() == reflect.Ptr {
		if val.IsNil() {
			// To get the type name from a nil pointer, we need to use Elem() on the type
			return fmt.Sprintf("%s{<nil>}", typ.Elem().Name())
		}
		val = val.Elem()
		typ = typ.Elem()
	}

	typeName := typ.Name()
	return fmt.Sprintf("%s{%+v}", typeName, val.Interface())
}
