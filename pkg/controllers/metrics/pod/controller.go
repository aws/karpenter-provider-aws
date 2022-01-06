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

package pod

import (
	"context"
	"fmt"
	"strings"

	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	podName             = "name"
	podNameSpace        = "namespace"
	ownerSelfLink       = "owner"
	podHostName         = "node"
	podProvisioner      = "provisioner"
	podHostZone         = "zone"
	podHostArchitecture = "arch"
	podHostCapacityType = "capacity_type"
	podHostInstanceType = "instance_type"
	podPhase            = "phase"
)

var (
	podGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "pods",
			Name:      "state",
			Help:      "Pod state.",
		},
		labelNames(),
	)
)

// Controller for the resource
type Controller struct {
	KubeClient client.Client
	LabelsMap  map[types.NamespacedName]prometheus.Labels
}

func init() {
	crmetrics.Registry.MustRegister(podGaugeVec)
}

func labelNames() []string {
	return []string{
		podName,
		podNameSpace,
		ownerSelfLink,
		podHostName,
		podProvisioner,
		podHostZone,
		podHostArchitecture,
		podHostCapacityType,
		podHostInstanceType,
		podPhase,
	}
}

// NewController constructs a controller instance
func NewController(kubeClient client.Client) *Controller {
	return &Controller{
		KubeClient: kubeClient,
		LabelsMap:  make(map[types.NamespacedName]prometheus.Labels),
	}
}

// Reconcile executes a termination control loop for the resource
func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("podmetrics").With("pod", req.Name))
	// Remove the previous gauge after pod labels are updated
	if labels, ok := c.LabelsMap[req.NamespacedName]; ok {
		podGaugeVec.Delete(labels)
	}
	// Retrieve pod from reconcile request
	pod := &v1.Pod{}
	if err := c.KubeClient.Get(ctx, req.NamespacedName, pod); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	c.record(ctx, pod)
	return reconcile.Result{}, nil
}

func (c *Controller) record(ctx context.Context, pod *v1.Pod) {
	labels := c.labels(ctx, pod)
	podGaugeVec.With(labels).Set(float64(1))
	c.LabelsMap[client.ObjectKeyFromObject(pod)] = labels
}

func (c *Controller) Register(ctx context.Context, m manager.Manager) error {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named("podmetrics").
		For(&v1.Pod{}).
		Complete(c)
}

// labels creates the labels using the current state of the pod
func (c *Controller) labels(ctx context.Context, pod *v1.Pod) prometheus.Labels {
	metricLabels := prometheus.Labels{}
	metricLabels[podName] = pod.GetName()
	metricLabels[podNameSpace] = pod.GetNamespace()
	// Selflink has been deprecated after v.1.20
	// Manually generate the selflink for the first owner reference
	// Currently we do not support multiple owner references
	selflink := ""
	if len(pod.GetOwnerReferences()) > 0 {
		ownerreference := pod.GetOwnerReferences()[0]
		selflink = fmt.Sprintf("/apis/%s/namespaces/%s/%ss/%s", ownerreference.APIVersion, pod.Namespace, strings.ToLower(ownerreference.Kind), ownerreference.Name)
	}
	metricLabels[ownerSelfLink] = selflink
	metricLabels[podHostName] = pod.Spec.NodeName
	metricLabels[podPhase] = string(pod.Status.Phase)
	node := &v1.Node{}
	if err := c.KubeClient.Get(ctx, types.NamespacedName{Name: pod.Spec.NodeName}, node); err != nil {
		metricLabels[podHostZone] = "N/A"
		metricLabels[podHostArchitecture] = "N/A"
		metricLabels[podHostCapacityType] = "N/A"
		metricLabels[podHostInstanceType] = "N/A"
		if provisionerName, ok := pod.Spec.NodeSelector[v1alpha5.ProvisionerNameLabelKey]; ok {
			metricLabels[podProvisioner] = provisionerName
		} else {
			metricLabels[podProvisioner] = "N/A"
		}
	} else {
		metricLabels[podHostZone] = node.Labels[v1.LabelTopologyZone]
		metricLabels[podHostArchitecture] = node.Labels[v1.LabelArchStable]
		if capacityType, ok := node.Labels[v1alpha5.LabelCapacityType]; !ok {
			metricLabels[podHostCapacityType] = "N/A"
		} else {
			metricLabels[podHostCapacityType] = capacityType
		}
		metricLabels[podHostInstanceType] = node.Labels[v1.LabelInstanceTypeStable]
		if provisionerName, ok := node.Labels[v1alpha5.ProvisionerNameLabelKey]; !ok {
			metricLabels[podProvisioner] = "N/A"
		} else {
			metricLabels[podProvisioner] = provisionerName
		}
	}
	return metricLabels
}
