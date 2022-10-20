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

func (d *dedupe) WaitingOnDeletionForConsolidation(node *v1.Node) {
	if !d.shouldCreateEvent(fmt.Sprintf("wait-node-consolidate-delete-%s", node.UID)) {
		return
	}
	d.rec.WaitingOnDeletionForConsolidation(node)
}

func (d *dedupe) WaitingOnReadinessForConsolidation(node *v1.Node) {
	if !d.shouldCreateEvent(fmt.Sprintf("wait-node-consolidate-ready-%s", node.UID)) {
		return
	}
	d.rec.WaitingOnReadinessForConsolidation(node)
}

func (d *dedupe) TerminatingNodeForConsolidation(node *v1.Node, reason string) {
	if !d.shouldCreateEvent(fmt.Sprintf("terminate-node-consolidate-%s-%s", node.UID, reason)) {
		return
	}
	d.rec.TerminatingNodeForConsolidation(node, reason)
}

func (d *dedupe) LaunchingNodeForConsolidation(node *v1.Node, reason string) {
	if !d.shouldCreateEvent(fmt.Sprintf("launch-node-consolidate-%s-%s", node.UID, reason)) {
		return
	}
	d.rec.LaunchingNodeForConsolidation(node, reason)
}

func (d *dedupe) NominatePod(pod *v1.Pod, node *v1.Node) {
	if !d.shouldCreateEvent(fmt.Sprintf("nominate-node-%s-%s", pod.UID, node.UID)) {
		return
	}
	d.rec.NominatePod(pod, node)
}

func (d *dedupe) EvictPod(pod *v1.Pod) {
	key := fmt.Sprintf("evict-pod-%s", pod.Name)
	if _, exists := d.cache.Get(key); exists {
		return
	}
	d.cache.SetDefault(key, nil)
	d.rec.EvictPod(pod)
}

func (d *dedupe) PodFailedToSchedule(pod *v1.Pod, err error) {
	if !d.shouldCreateEvent(fmt.Sprintf("failed-to-schedule-%s-%s", pod.UID, err)) {
		return
	}
	d.rec.PodFailedToSchedule(pod, err)
}

func (d *dedupe) NodeFailedToDrain(node *v1.Node, err error) {
	if !d.shouldCreateEvent(fmt.Sprintf("failed-to-drain-%s", node.Name)) {
		return
	}
	d.rec.NodeFailedToDrain(node, err)
}

func (d *dedupe) shouldCreateEvent(key string) bool {
	if _, exists := d.cache.Get(key); exists {
		return false
	}
	d.cache.SetDefault(key, nil)
	return true
}
