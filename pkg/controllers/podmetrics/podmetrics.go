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

package podmetrics

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
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	podname             = "name"
	podnamespace        = "namespace"
	ownerselflink       = "owner"
	podhostname         = "node"
	podprovisioner      = "provisioner"
	podhostzone         = "zone"
	podhostarchitecture = "arch"
	podhostcapacitype   = "capacitytype"
	podhostinstancetype = "instancetype"
	podphase            = "phase"
)

var (
	podGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "pods",
			Name:      "podstate",
			Help:      "Pod state.",
		},
		getLabelNames(),
	)
)

// Controller for the resource
type Controller struct {
	KubeClient   client.Client
	CoreV1Client corev1.CoreV1Interface
	LabelsMap    map[types.NamespacedName]*prometheus.Labels
}

func init() {
	crmetrics.Registry.MustRegister(podGaugeVec)
}

func getLabelNames() []string {
	return []string{
		podname,
		podnamespace,
		ownerselflink,
		podhostname,
		podprovisioner,
		podhostzone,
		podhostarchitecture,
		podhostcapacitype,
		podhostinstancetype,
		podphase,
	}

}

// NewController constructs a controller instance
func NewController(kubeClient client.Client, coreV1Client corev1.CoreV1Interface) *Controller {
	newcontroller := Controller{
		KubeClient:   kubeClient,
		CoreV1Client: coreV1Client,
		LabelsMap:    make(map[types.NamespacedName]*prometheus.Labels),
	}
	return &newcontroller
}

// Reconcile executes a termination control loop for the resource
func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("podmetrics").With("pod", req.Name))

	logging.FromContext(ctx).Infof("=================== Monitoring Pods %s ====================", req.NamespacedName)

	// Retrieve pod from reconcile request
	pod := &v1.Pod{}
	if err := c.KubeClient.Get(ctx, req.NamespacedName, pod); err != nil {
		if errors.IsNotFound(err) {
			// Remove gauge due to pod deletion
			if labels, ok := c.LabelsMap[req.NamespacedName]; ok {
				podGaugeVec.Delete(*labels)
			} else {
				logging.FromContext(ctx).Errorf("Failed to delete gauge: failed to locate labels")
			}
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	// Remove the previous gauge after pod labels are updated
	if labels, ok := c.LabelsMap[req.NamespacedName]; ok {
		podGaugeVec.Delete(*labels)
	}
	newlabels, err := c.generateLabels(ctx, pod)
	if err != nil {
		logging.FromContext(ctx).Errorf("Failed to generate prometheus labels: %s", err.Error())
		return reconcile.Result{}, err
	}

	gauge, err := podGaugeVec.GetMetricWith(*newlabels)
	if err != nil {
		logging.FromContext(ctx).Errorf("Failed to generate new gauge: %s", err.Error())
		return reconcile.Result{}, err
	}

	logging.FromContext(ctx).Infof("Pod Phase: %s, Pod Condition: %s", string(pod.Status.Phase), &pod.Status.Conditions)
	gauge.Set(float64(1))
	c.LabelsMap[req.NamespacedName] = newlabels

	return reconcile.Result{}, nil
}

func (c *Controller) Register(ctx context.Context, m manager.Manager) error {
	err := controllerruntime.
		NewControllerManagedBy(m).
		Named("podmetrics").
		For(&v1.Pod{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10000}).
		Complete(c)
	return err
}

// generateLabels creates the labels using the current state of the pod
func (c *Controller) generateLabels(ctx context.Context, pod *v1.Pod) (*prometheus.Labels, error) {
	metricLabels := prometheus.Labels{}
	metricLabels[podname] = pod.GetName()
	metricLabels[podnamespace] = pod.GetNamespace()
	// Selflink has been deprecated after v.1.20
	// Manually generate the selflink for the first owner reference
	// Currently we do not support multiple over references
	selflink := ""
	if len(pod.GetOwnerReferences()) != 0 {
		ownerreference := pod.GetOwnerReferences()[0]
		selflink = fmt.Sprintf("/apis/%s/namespaces/%s/%ss/%s", ownerreference.APIVersion, pod.Namespace, strings.ToLower(ownerreference.Kind), ownerreference.Name)
	}
	metricLabels[ownerselflink] = selflink
	metricLabels[podhostname] = pod.Spec.NodeName
	metricLabels[podphase] = string(pod.Status.Phase)
	provisioner := v1alpha5.DefaultProvisioner
	if name, ok := pod.Spec.NodeSelector[v1alpha5.ProvisionerNameLabelKey]; ok {
		provisioner.Name = name
	}
	metricLabels[podprovisioner] = provisioner.Name
	nodename := types.NamespacedName{Name: pod.Spec.NodeName}
	node := &v1.Node{}
	if err := c.KubeClient.Get(ctx, nodename, node); err != nil {
		return nil, fmt.Errorf("getting node for pod, %w", err)
	}
	metricLabels[podhostzone] = node.Labels[v1.LabelTopologyZone]
	metricLabels[podhostarchitecture] = node.Labels[v1.LabelArchStable]
	metricLabels[podhostcapacitype] = node.Labels[v1alpha5.LabelCapacityType]
	metricLabels[podhostinstancetype] = node.Labels[v1.LabelInstanceTypeStable]
	return &metricLabels, nil
}
