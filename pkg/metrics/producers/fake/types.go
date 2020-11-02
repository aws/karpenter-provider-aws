package fake

import "fmt"

var (
	NotImplementedError           = fmt.Errorf("provider is not implemented")
	InvalidQueueError             = fmt.Errorf("queue inputs are invalid")
	InvalidPendingCapacityError   = fmt.Errorf("pending capacity inputs are invalid")
	InvalidReservedCapacityError  = fmt.Errorf("reserved capacity inputs are invalid")
	InvalidScheduledCapacityError = fmt.Errorf("scheduled capacity inputs are invalid")
)

// FakeProducer is a noop implementation
type FakeProducer struct {
	Error error
}

func (p *FakeProducer) Reconcile() error { return p.Error }
