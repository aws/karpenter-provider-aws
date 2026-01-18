/*
Copyright 2018 The Kubernetes Authors.

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

package signals

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("runtime signal", func() {

	Context("SignalHandler Test", func() {

		It("test signal handler", func() {
			ctx := SetupSignalHandler()
			task := &Task{
				ticker: time.NewTicker(time.Second * 2),
			}
			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt)
			task.wg.Add(1)
			go func(c chan os.Signal) {
				defer task.wg.Done()
				task.Run(c)
			}(c)

			select {
			case sig := <-c:
				fmt.Printf("Got %s signal. Aborting...\n", sig)
			case _, ok := <-ctx.Done():
				Expect(ok).To(BeFalse())
			}
		})

	})

})

type Task struct {
	wg     sync.WaitGroup
	ticker *time.Ticker
}

func (t *Task) Run(c chan os.Signal) {
	for {
		go sendSignal(c)
		handle()
	}
}

func handle() {
	for i := 0; i < 5; i++ {
		fmt.Print("#")
		time.Sleep(time.Millisecond * 100)
	}
	fmt.Println()
}

func sendSignal(stopChan chan os.Signal) {
	fmt.Printf("...")
	time.Sleep(1 * time.Second)
	stopChan <- os.Interrupt
}
