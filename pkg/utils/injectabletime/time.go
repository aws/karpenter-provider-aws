package injectabletime

import "time"

// Now is a time.Now() that may be mocked by tests.
var Now = time.Now
