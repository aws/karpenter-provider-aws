package parallel

import (
	"sync"

	"golang.org/x/time/rate"
	"k8s.io/client-go/util/workqueue"
)

type WorkQueue struct {
	workqueue.RateLimitingInterface
	once    sync.Once
}

func NewWorkQueue(qps int, burst int) *WorkQueue {
	return &WorkQueue{
		RateLimitingInterface: workqueue.NewRateLimitingQueue(&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(qps), burst)}),
	}
}

type Task struct {
	do   func() error
	done chan error
	err  error
}

// Add work to the queue, returns a channel that signals when the work is done.
func (c *WorkQueue) Add(do func() error) chan error {
	c.once.Do(c.start)
	done := make(chan error)
	c.AddRateLimited(&Task{do: do, done: done})
	return done
}

func (c *WorkQueue) start() {
	go func() {
		for {
			item, shutdown := c.Get()
			if shutdown {
				break
			}
			task := item.(*Task)
			go func() {
				task.err = task.do()
				c.Forget(task)
				c.Done(task)
				task.done <- task.err
			}()
		}
	}()
}
