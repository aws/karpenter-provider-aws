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

package controllers

import (
	"context"

	"github.com/awslabs/operatorpkg/controller"
	"github.com/awslabs/operatorpkg/object"
	"github.com/awslabs/operatorpkg/status"
	"github.com/samber/lo"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	corev1 "k8s.io/api/core/v1"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/controllers/disruption"
	"sigs.k8s.io/karpenter/pkg/controllers/disruption/orchestration"
	metricsnode "sigs.k8s.io/karpenter/pkg/controllers/metrics/node"
	metricsnodepool "sigs.k8s.io/karpenter/pkg/controllers/metrics/nodepool"
	metricspod "sigs.k8s.io/karpenter/pkg/controllers/metrics/pod"
	"sigs.k8s.io/karpenter/pkg/controllers/node/health"
	nodehydration "sigs.k8s.io/karpenter/pkg/controllers/node/hydration"
	"sigs.k8s.io/karpenter/pkg/controllers/node/termination"
	"sigs.k8s.io/karpenter/pkg/controllers/node/termination/terminator"
	nodeclaimconsistency "sigs.k8s.io/karpenter/pkg/controllers/nodeclaim/consistency"
	nodeclaimdisruption "sigs.k8s.io/karpenter/pkg/controllers/nodeclaim/disruption"
	"sigs.k8s.io/karpenter/pkg/controllers/nodeclaim/expiration"
	nodeclaimgarbagecollection "sigs.k8s.io/karpenter/pkg/controllers/nodeclaim/garbagecollection"
	nodeclaimhydration "sigs.k8s.io/karpenter/pkg/controllers/nodeclaim/hydration"
	nodeclaimlifecycle "sigs.k8s.io/karpenter/pkg/controllers/nodeclaim/lifecycle"
	"sigs.k8s.io/karpenter/pkg/controllers/nodeclaim/podevents"
	nodepoolcounter "sigs.k8s.io/karpenter/pkg/controllers/nodepool/counter"
	nodepoolhash "sigs.k8s.io/karpenter/pkg/controllers/nodepool/hash"
	nodepoolreadiness "sigs.k8s.io/karpenter/pkg/controllers/nodepool/readiness"
	nodepoolvalidation "sigs.k8s.io/karpenter/pkg/controllers/nodepool/validation"
	"sigs.k8s.io/karpenter/pkg/controllers/provisioning"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/controllers/state/informer"
	"sigs.k8s.io/karpenter/pkg/events"
	"sigs.k8s.io/karpenter/pkg/operator/options"
)

func NewControllers(
	ctx context.Context,
	mgr manager.Manager,
	clock clock.Clock,
	kubeClient client.Client,
	recorder events.Recorder,
	cloudProvider cloudprovider.CloudProvider,
	cluster *state.Cluster,
) []controller.Controller {
	p := provisioning.NewProvisioner(kubeClient, recorder, cloudProvider, cluster, clock)
	evictionQueue := terminator.NewQueue(kubeClient, recorder)
	disruptionQueue := orchestration.NewQueue(kubeClient, recorder, cluster, clock, p)

	controllers := []controller.Controller{
		p, evictionQueue, disruptionQueue,
		disruption.NewController(clock, kubeClient, p, cloudProvider, recorder, cluster, disruptionQueue),
		provisioning.NewPodController(kubeClient, p, cluster),
		provisioning.NewNodeController(kubeClient, p),
		nodepoolhash.NewController(kubeClient, cloudProvider),
		expiration.NewController(clock, kubeClient, cloudProvider),
		informer.NewDaemonSetController(kubeClient, cluster),
		informer.NewNodeController(kubeClient, cluster),
		informer.NewPodController(kubeClient, cluster),
		informer.NewNodePoolController(kubeClient, cloudProvider, cluster),
		informer.NewNodeClaimController(kubeClient, cloudProvider, cluster),
		termination.NewController(clock, kubeClient, cloudProvider, terminator.NewTerminator(clock, kubeClient, evictionQueue, recorder), recorder),
		metricspod.NewController(kubeClient, cluster),
		metricsnodepool.NewController(kubeClient, cloudProvider),
		metricsnode.NewController(cluster),
		nodepoolreadiness.NewController(kubeClient, cloudProvider),
		nodepoolcounter.NewController(kubeClient, cloudProvider, cluster),
		nodepoolvalidation.NewController(kubeClient, cloudProvider),
		podevents.NewController(clock, kubeClient, cloudProvider),
		nodeclaimconsistency.NewController(clock, kubeClient, cloudProvider, recorder),
		nodeclaimlifecycle.NewController(clock, kubeClient, cloudProvider, recorder),
		nodeclaimgarbagecollection.NewController(clock, kubeClient, cloudProvider),
		nodeclaimdisruption.NewController(clock, kubeClient, cloudProvider),
		nodeclaimhydration.NewController(kubeClient, cloudProvider),
		nodehydration.NewController(kubeClient, cloudProvider),
		status.NewController[*v1.NodeClaim](kubeClient, mgr.GetEventRecorderFor("karpenter"), status.EmitDeprecatedMetrics, status.WithLabels(append(lo.Map(cloudProvider.GetSupportedNodeClasses(), func(obj status.Object, _ int) string { return v1.NodeClassLabelKey(object.GVK(obj).GroupKind()) }), v1.NodePoolLabelKey)...)),
		status.NewController[*v1.NodePool](kubeClient, mgr.GetEventRecorderFor("karpenter"), status.EmitDeprecatedMetrics),
		status.NewGenericObjectController[*corev1.Node](kubeClient, mgr.GetEventRecorderFor("karpenter"), status.WithLabels(append(lo.Map(cloudProvider.GetSupportedNodeClasses(), func(obj status.Object, _ int) string { return v1.NodeClassLabelKey(object.GVK(obj).GroupKind()) }), v1.NodePoolLabelKey, v1.NodeInitializedLabelKey)...)),
	}

	// The cloud provider must define status conditions for the node repair controller to use to detect unhealthy nodes
	if len(cloudProvider.RepairPolicies()) != 0 && options.FromContext(ctx).FeatureGates.NodeRepair {
		controllers = append(controllers, health.NewController(kubeClient, cloudProvider, clock, recorder))
	}

	return controllers
}
