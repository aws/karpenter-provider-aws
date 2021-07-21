package parallel

import (
	"sync"

	"golang.org/x/time/rate"
	"k8s.io/client-go/util/workqueue"
)

// WorkQueue is a thread safe task runner
type WorkQueue struct {
	workqueue.RateLimitingInterface
	once    sync.Once
}

// NewWorkQueue instantiates a new WorkQueue
func NewWorkQueue(qps int, burst int) *WorkQueue {
	return &WorkQueue{
		RateLimitingInterface: workqueue.NewRateLimitingQueue(&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(qps), burst)}),
	}
}

type task struct {
	do   func() error
	done chan error
	err  error
}

// Add work to the queue, returns a channel that signals when the work is done.
func (c *WorkQueue) Add(do func() error) chan error {
	c.once.Do(c.start)
	done := make(chan error)
	c.AddRateLimited(&task{do: do, done: done})
	return done
}

func (c *WorkQueue) start() {
	go func() {
		for {
			item, shutdown := c.Get()
			if shutdown {
				break
			}
			t := item.(*task)
			go func() {
				t.err = t.do()
				c.Forget(t)
				c.Done(t)
				t.done <- t.err
			}()
		}
	}()
}
