package htcat

import "fmt"

// Raised if an internal API
type ErrAssertf struct {
	error
}

func AssertErrf(format string, a ...interface{}) ErrAssertf {
	return ErrAssertf{fmt.Errorf(format, a...)}
}
