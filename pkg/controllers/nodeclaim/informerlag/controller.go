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

package informerlag

import (
	"context"
	"fmt"
	"time"

	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
)

// Controller measures how far the NodeClaim informer cache trails the API server. It observes each update event
// and doesn't reconcile.
type Controller struct {
	clock clock.Clock
}

func NewController(clk clock.Clock) *Controller {
	return &Controller{clock: clk}
}

func (c *Controller) Name() string {
	return "nodeclaim.informerlag"
}

func (c *Controller) Register(ctx context.Context, m manager.Manager) error {
	// Lag reads managedFields, so this breaks if the NodeClaim cache starts stripping them like core does for Services
	informer, err := m.GetCache().GetInformer(ctx, &karpv1.NodeClaim{})
	if err != nil {
		return fmt.Errorf("getting nodeclaim informer, %w", err)
	}
	if _, err = informer.AddEventHandler(c.Handler()); err != nil {
		return fmt.Errorf("adding nodeclaim informer handler, %w", err)
	}
	return nil
}

func (c *Controller) Handler() toolscache.ResourceEventHandlerFuncs {
	return toolscache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj any) {
			if lag, ok := Lag(c.clock.Now(), oldObj, newObj); ok {
				LagSeconds.Observe(lag.Seconds(), map[string]string{})
			}
		},
	}
}

// Lag returns the age of the write that produced an update event, taken from the newest managedFields time. The API
// server sets an entry's time when its manager adds or changes a field. Writes that only remove fields, such as a
// finalizer removal, and setting the deletionTimestamp leave every time unchanged. Lag skips updates whose newest
// time didn't advance, which also skips resyncs, so it doesn't measure from an older write.
func Lag(now time.Time, oldObj, newObj any) (time.Duration, bool) {
	oldNodeClaim, ok := oldObj.(*karpv1.NodeClaim)
	if !ok {
		return 0, false
	}
	newNodeClaim, ok := newObj.(*karpv1.NodeClaim)
	if !ok {
		return 0, false
	}
	written := lastWrite(newNodeClaim)
	if !written.After(lastWrite(oldNodeClaim)) {
		return 0, false
	}
	return max(now.Sub(written), 0), true
}

func lastWrite(nodeClaim *karpv1.NodeClaim) time.Time {
	var written time.Time
	for _, entry := range nodeClaim.ManagedFields {
		if entry.Time != nil && entry.Time.After(written) {
			written = entry.Time.Time
		}
	}
	return written
}
