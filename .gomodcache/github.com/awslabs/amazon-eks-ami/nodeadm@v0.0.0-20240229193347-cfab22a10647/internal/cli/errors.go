package cli

import "fmt"

// ErrMustRunAsRoot is returned when a command must be run as root.
var ErrMustRunAsRoot = fmt.Errorf("must run as root")
