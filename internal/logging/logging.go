// Package logging defines the Logger interface used across the project and
// helpers for formatting values in log output.
package logging

import (
	"fmt"
	"reflect"
)

type Logger interface {
	Tracef(f string, a ...any)
	Debugf(f string, a ...any)
	Infof(f string, a ...any)
	Errorf(f string, a ...any)
	Fatalf(f string, a ...any)
}

// Formatter interface for types that can format themselves for logging
type Formatter interface {
	Format() string
}

// FormatValue uses Formatter interface if available, otherwise returns default format
func FormatValue[T any](v T) string {
	if formatter, ok := any(v).(Formatter); ok {
		return formatter.Format()
	}

	val := reflect.ValueOf(v)
	typ := reflect.TypeOf(v)

	if typ == nil {
		return "<nil>"
	}

	if typ.Kind() == reflect.Pointer {
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
