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

package reservedcapacity

import (
	"context"
	"fmt"
	"math"

	"github.com/awslabs/karpenter/pkg/metrics"

	"github.com/awslabs/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/awslabs/karpenter/pkg/utils/node"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Producer implements a Reserved Capacity metric
type Producer struct {
	*v1alpha1.MetricsProducer
	Client client.Client
}

// Reconcile of the metrics
func (p *Producer) Reconcile() error {
	// 1. List nodes
	nodes := &v1.NodeList{}
	if err := p.Client.List(context.Background(), nodes, client.MatchingLabels(p.Spec.ReservedCapacity.NodeSelector)); err != nil {
		return fmt.Errorf("Listing nodes for %s, %w", p.Spec.ReservedCapacity.NodeSelector, err)
	}

	// 2. Compute reservations
	reservations := NewReservations()
	for _, n := range nodes.Items {
		// Only count nodes that are ready and schedulable to avoid diluting the
		// denomenator with unschedulable nodes. This can lead to premature
		// scale down before the scheduler assigns pod to the node.
		if node.IsReadyAndSchedulable(n) {
			pods := &v1.PodList{}
			if err := p.Client.List(context.Background(), pods, client.MatchingFields{"spec.nodeName": n.Name}); err != nil {
				return fmt.Errorf("Listing pods for %s, %w", n.Name, err)
			}
			reservations.Add(&n, pods)
		}
	}

	// 3. Record reservations and update status
	p.record(reservations)
	return nil
}

func (p *Producer) record(reservations *Reservations) {
	if p.Status.ReservedCapacity == nil {
		p.Status.ReservedCapacity = map[v1.ResourceName]string{}
	}
	for resource, reservation := range reservations.Resources {
		computed := reservation.Compute()
		for metricType, value := range computed {
			metrics.Gauges[Subsystem][fmt.Sprintf("%s_%s", resource, metricType)].
				WithLabelValues(p.Name, p.Namespace).Set(value)
		}
		p.Status.ReservedCapacity[resource] = fmt.Sprintf(
			"%v%%, %d/%d",
			math.Floor(computed[Utilization]*100),
			int64(computed[Reserved]),
			int64(computed[Capacity]),
		)
	}
}
