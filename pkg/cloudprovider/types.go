package cloudprovider

// Queue abstracts all provider specific behavior for Queues
type Queue interface {
	// Name returns the name of the queue
	Name() string
	// Length returns the length of the queue
	Length() (int64, error)
	// OldestMessageAge returns the age of the oldest message
	OldestMessageAge() (int64, error)
}

// NodeGroup abstracts all provider specific behavior for NodeGroups.
type NodeGroup interface {
	// Reconcile sets the NodeGroups's replica count if possible, and
	// also always tries to update status
	Reconcile() error
}
