// Package exercises contains the learner's code. These functions deliberately
// start unfinished. The experiment runner calls them without reference fallbacks.
package exercises

import "fmt"

// Unimplemented identifies an unfinished exercise to the CLI and tests.
// Replace each panic with your implementation; leave this helper as it is.
type Unimplemented struct {
	Lesson   int
	Function string
}

func (u Unimplemented) Error() string {
	if u.Lesson == 0 {
		return fmt.Sprintf("%s is unfinished; edit exercises/sandbox.go", u.Function)
	}
	return fmt.Sprintf("%s is unfinished; run audiolab lesson %d", u.Function, u.Lesson)
}
