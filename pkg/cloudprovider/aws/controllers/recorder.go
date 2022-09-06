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

package controllers

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

type recorder struct {
	record.EventRecorder
}

type Recorder interface {
	record.EventRecorder

	// EC2SpotInterruptionWarning is called when EC2 sends a spot interruption 2-minute warning for the node from the SQS queue
	EC2SpotInterruptionWarning(*v1.Node)
	// EC2SpotRebalanceRecommendation is called when EC2 sends a rebalance recommendation for the node from the SQS queue
	EC2SpotRebalanceRecommendation(*v1.Node)
	// EC2HealthWarning is called when EC2 sends a health warning notification for a health issue for the node from the SQS queue
	EC2HealthWarning(*v1.Node)
}

func NewRecorder(r record.EventRecorder) Recorder {
	return recorder{
		EventRecorder: r,
	}
}

func (r recorder) EC2SpotInterruptionWarning(node *v1.Node) {
	r.Eventf(node, "Normal", "EC2SpotInterruptionWarning", "Node %s event: EC2 triggered a spot interruption warning for the node", node.Name)
}

func (r recorder) EC2SpotRebalanceRecommendation(node *v1.Node) {
	r.Eventf(node, "Normal", "EC2RebalanceRecommendation", "Node %s event: EC2 triggered a spot rebalance recommendation for the node", node.Name)
}

func (r recorder) EC2HealthWarning(node *v1.Node) {
	r.Eventf(node, "Normal", "EC2HealthWarning", "Node %s event: EC2 triggered a health warning for the node", node.Name)
}
