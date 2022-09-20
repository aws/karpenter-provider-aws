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
	"context"

	"github.com/avast/retry-go"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/events"
	"github.com/aws/karpenter/pkg/utils/injection"
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
	// TerminatingNodeOnNotification is called when a notification that is sent to the notification controller triggers node deletion
	TerminatingNodeOnNotification(*v1.Node)
	// InfrastructureUnhealthy event is called when infrastructure reconciliation errors and the controller enters an unhealthy state
	InfrastructureUnhealthy(context.Context, client.Client)
	// InfrastructureHealthy event is called when infrastructure reconciliation succeeds and the controller enters a healthy state
	InfrastructureHealthy(context.Context, client.Client)
}

func NewRecorder(r events.Recorder) Recorder {
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

func (r recorder) TerminatingNodeOnNotification(node *v1.Node) {
	r.Eventf(node, "Normal", "NotificationTerminateNode", "Node %s event: Notification triggered termination for the node", node.Name)
}

func (r recorder) InfrastructureHealthy(ctx context.Context, kubeClient client.Client) {
	dep := &appsv1.Deployment{}
	err := retry.Do(func() error {
		return kubeClient.Get(ctx, types.NamespacedName{Namespace: injection.GetOptions(ctx).DeploymentNamespace, Name: injection.GetOptions(ctx).DeploymentName}, dep)
	})
	if err != nil {
		logging.FromContext(ctx).Errorf("Sending InfrastructureHealthy event, %v", err)
		return
	}
	r.Eventf(dep, "Normal", "InfrastructureHealthy", "Karpenter infrastructure reconciliation is healthy")
}

func (r recorder) InfrastructureUnhealthy(ctx context.Context, kubeClient client.Client) {
	dep := &appsv1.Deployment{}
	err := retry.Do(func() error {
		return kubeClient.Get(ctx, types.NamespacedName{Namespace: injection.GetOptions(ctx).DeploymentNamespace, Name: injection.GetOptions(ctx).DeploymentName}, dep)
	})
	if err != nil {
		logging.FromContext(ctx).Errorf("Sending InfrastructureUnhealthy event, %v", err)
		return
	}
	r.Eventf(dep, "Warning", "InfrastructureUnhealthy", "Karpenter infrastructure reconciliation is unhealthy")
}
