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

package events

import (
	"fmt"
	"time"

	"github.com/patrickmn/go-cache"
	v1 "k8s.io/api/core/v1"
)

func NewDedupeRecorder(r Recorder) Recorder {
	return &dedupe{
		rec:   r,
		cache: cache.New(120*time.Second, 10*time.Second),
	}
}

type dedupe struct {
	rec   Recorder
	cache *cache.Cache
}

func (d *dedupe) NominatePod(pod *v1.Pod, node *v1.Node) {
	key := fmt.Sprintf("nominate-node-%s-%s", pod.Name, node.Name)
	if _, exists := d.cache.Get(key); exists {
		return
	}
	d.cache.SetDefault(key, nil)
	d.rec.NominatePod(pod, node)
}

func (d *dedupe) PodFailedToSchedule(pod *v1.Pod, err error) {
	key := fmt.Sprintf("failed-to-schedule-%s-%s", pod.Name, err.Error())
	if _, exists := d.cache.Get(key); exists {
		return
	}
	d.cache.SetDefault(key, nil)
	d.rec.PodFailedToSchedule(pod, err)
}
