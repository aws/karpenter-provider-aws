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

package test

import (
	"sync"
	"time"

	"github.com/aws/karpenter/pkg/config"
)

type Config struct {
	Mu                sync.Mutex
	Handlers          []config.ChangeHandler
	batchMaxDuration  time.Duration
	batchIdleDuration time.Duration
}

func (c *Config) OnChange(handler config.ChangeHandler) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	c.Handlers = append(c.Handlers, handler)
}

func (c *Config) SetBatchMaxDuration(d time.Duration) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	c.batchMaxDuration = d
}
func (c *Config) BatchMaxDuration() time.Duration {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return c.batchMaxDuration
}

func (c *Config) SetBatchIdleDuration(d time.Duration) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	c.batchIdleDuration = d
}
func (c *Config) BatchIdleDuration() time.Duration {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return c.batchIdleDuration
}

func NewConfig() *Config {
	return &Config{
		batchMaxDuration:  10 * time.Second,
		batchIdleDuration: 1 * time.Second,
	}
}
