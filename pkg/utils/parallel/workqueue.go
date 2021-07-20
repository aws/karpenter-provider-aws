/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package parallel

import (
	"sync"

	"golang.org/x/time/rate"
	"k8s.io/client-go/util/workqueue"
)

// WorkQueue is a thread safe task runner
type WorkQueue struct {
	workqueue.RateLimitingInterface
	once sync.Once
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
