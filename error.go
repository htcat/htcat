package htcat

import "fmt"

// Returned if an internal fidelity check fails.
type errAssertf struct {
	error
}

func assertErrf(format string, a ...interface{}) errAssertf {
	return errAssertf{fmt.Errorf(format, a...)}
}
