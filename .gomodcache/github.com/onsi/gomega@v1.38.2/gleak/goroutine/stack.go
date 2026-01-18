package goroutine

import "runtime"

const startStackBufferSize = 64 * 1024 // 64kB

// stacks returns stack trace information for either all goroutines or only the
// current goroutine. It is a convenience wrapper around runtime.Stack, hiding
// the result allocation.
func stacks(all bool) []byte {
	for size := startStackBufferSize; ; size *= 2 {
		buffer := make([]byte, size)
		if n := runtime.Stack(buffer, all); n < size {
			return buffer[:n]
		}
	}
}
