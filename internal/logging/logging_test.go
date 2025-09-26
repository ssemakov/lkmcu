package logging

import (
	"fmt"
	"testing"
)

type formattable struct{ A int }
func (f formattable) Format() string { return fmt.Sprintf("X=%d", f.A) }

type S struct{ A int }

func TestFormatValue_Formatter(t *testing.T) {
    got := FormatValue(formattable{A: 1})
    if got != "X=1" {
        t.Fatalf("got %q, want %q", got, "X=1")
    }
}

func TestFormatValue_Struct(t *testing.T) {
    got := FormatValue(S{A: 2})
    want := "S{"+fmt.Sprintf("%+v", S{A:2})+"}"
    if got != want { t.Fatalf("got %q, want %q", got, want) }
}

func TestFormatValue_PtrStruct(t *testing.T) {
    s := &S{A: 3}
    got := FormatValue(s)
    want := "S{"+fmt.Sprintf("%+v", *s)+"}"
    if got != want { t.Fatalf("got %q, want %q", got, want) }
}

func TestFormatValue_NilPtrStruct(t *testing.T) {
    var s *S // nil
    got := FormatValue(s)
    if got != "S{<nil>}" {
        t.Fatalf("got %q, want %q", got, "S{<nil>}")
    }
}

func TestFormatValue_Builtin(t *testing.T) {
    if got := FormatValue(42); got != "int{42}" { t.Fatalf("got %q", got) }
    if got := FormatValue("hi"); got != "string{hi}" { t.Fatalf("got %q", got) }
}

// Optional: document or fix this behavior before enabling the test.
func TestFormatValue_TypedNilInterface(t *testing.T) {
    var v any = nil
    got := FormatValue(v)
		if got != "<nil>" {
				t.Fatalf("got %q, want %q", got, "<nil>")
		}
}
