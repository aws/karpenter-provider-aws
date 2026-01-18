package syncs

import (
	"testing"
	"time"

	// This appears to be needed so that the prow test runner won't fail.
	_ "github.com/onsi/ginkgo/v2"
	_ "github.com/onsi/gomega"
)

func TestMergeChans(t *testing.T) {
	tests := []struct {
		name   string
		count  int
		signal int
	}{
		{
			name:   "single channel, close 0",
			count:  1,
			signal: 0,
		},
		{
			name:   "double channel, close 0",
			count:  2,
			signal: 0,
		},
		{
			name:   "five channel, close 0",
			count:  5,
			signal: 0,
		},
		{
			name:   "five channel, close 1",
			count:  5,
			signal: 1,
		},
		{
			name:   "five channel, close 2",
			count:  5,
			signal: 2,
		},
		{
			name:   "five channel, close 3",
			count:  5,
			signal: 3,
		},
		{
			name:   "five channel, close 4",
			count:  5,
			signal: 4,
		},
		{
			name:   "single channel, cancel",
			count:  1,
			signal: -1,
		},
		{
			name:   "double channel, cancel",
			count:  2,
			signal: -1,
		},
		{
			name:   "five channel, cancel",
			count:  5,
			signal: -1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if callAndClose(test.count, test.signal, 1) {
				t.Error("timeout before merged channel closed")
			}
		})
	}
}

func callAndClose(numChans, signalChan, timeoutSeconds int) bool {
	chans := make([]chan struct{}, numChans)
	readOnlyChans := make([]<-chan struct{}, numChans)
	for i := range chans {
		chans[i] = make(chan struct{})
		readOnlyChans[i] = chans[i]
	}
	defer func() {
		for i := range chans {
			close(chans[i])
		}
	}()

	merged, cancel := MergeChans(readOnlyChans...)
	defer cancel()

	timer := time.NewTimer(time.Duration(timeoutSeconds) * time.Second)

	if signalChan >= 0 {
		chans[signalChan] <- struct{}{}
	} else {
		cancel()
	}
	select {
	case <-merged:
		return false
	case <-timer.C:
		return true
	}
}
