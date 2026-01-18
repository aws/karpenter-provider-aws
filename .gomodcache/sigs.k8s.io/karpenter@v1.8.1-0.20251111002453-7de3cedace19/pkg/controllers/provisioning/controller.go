/*
Copyright The Kubernetes Authors.

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

package provisioning

import (
	"context"
	"time"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	utilscontroller "sigs.k8s.io/karpenter/pkg/utils/controller"
	"sigs.k8s.io/karpenter/pkg/utils/pod"
)

const (
	minReconciles = 10
	maxReconciles = 1000
)

// PodController for the resource
type PodController struct {
	kubeClient  client.Client
	provisioner *Provisioner
	cluster     *state.Cluster
}

// NewPodController constructs a controller instance
func NewPodController(kubeClient client.Client, provisioner *Provisioner, cluster *state.Cluster) *PodController {
	return &PodController{
		kubeClient:  kubeClient,
		provisioner: provisioner,
		cluster:     cluster,
	}
}

// Reconcile the resource
func (c *PodController) Reconcile(ctx context.Context, p *corev1.Pod) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, "provisioner.trigger.pod") //nolint:ineffassign,staticcheck

	if !pod.IsProvisionable(p) {
		return reconcile.Result{}, nil
	}
	c.provisioner.Trigger(p.UID)
	// ACK the pending pod when first observed so that total time spent pending due to Karpenter is tracked.
	c.cluster.AckPods(p)
	// Continue to requeue until the pod is no longer provisionable. Pods may
	// not be scheduled as expected if new pods are created while nodes are
	// coming online. Even if a provisioning loop is successful, the pod may
	// require another provisioning loop to become schedulable.
	return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
}

func (c *PodController) Register(ctx context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("provisioner.trigger.pod").
		For(&corev1.Pod{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: utilscontroller.LinearScaleReconciles(utilscontroller.CPUCount(ctx), minReconciles, maxReconciles)}).
		Complete(reconcile.AsReconciler(m.GetClient(), c))
}

// NodeController for the resource
type NodeController struct {
	kubeClient  client.Client
	provisioner *Provisioner
}

// NewNodeController constructs a controller instance
func NewNodeController(kubeClient client.Client, provisioner *Provisioner) *NodeController {
	return &NodeController{
		kubeClient:  kubeClient,
		provisioner: provisioner,
	}
}

// Reconcile the resource
func (c *NodeController) Reconcile(ctx context.Context, n *corev1.Node) (reconcile.Result, error) {
	//nolint:ineffassign
	ctx = injection.WithControllerName(ctx, "provisioner.trigger.node") //nolint:ineffassign,staticcheck

	// If the disruption taint doesn't exist and the deletion timestamp isn't set, it's not being disrupted.
	// We don't check the deletion timestamp here, as we expect the termination controller to eventually set
	// the taint when it picks up the node from being deleted.
	if !lo.ContainsBy(n.Spec.Taints, func(taint corev1.Taint) bool {
		return taint.MatchTaint(&v1.DisruptedNoScheduleTaint)
	}) {
		return reconcile.Result{}, nil
	}
	c.provisioner.Trigger(n.UID)
	// Continue to requeue until the node is no longer provisionable. Pods may
	// not be scheduled as expected if new pods are created while nodes are
	// coming online. Even if a provisioning loop is successful, the pod may
	// require another provisioning loop to become schedulable.
	return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
}

func (c *NodeController) Register(ctx context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("provisioner.trigger.node").
		For(&corev1.Node{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: utilscontroller.LinearScaleReconciles(utilscontroller.CPUCount(ctx), minReconciles, maxReconciles)}).
		Complete(reconcile.AsReconciler(m.GetClient(), c))
}
