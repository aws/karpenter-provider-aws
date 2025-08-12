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

//nolint:revive
package expectations

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"regexp"
	"sync"
	"time"

	"k8s.io/utils/clock/testing"

	opmetrics "github.com/awslabs/operatorpkg/metrics"
	"github.com/awslabs/operatorpkg/singleton"
	"github.com/awslabs/operatorpkg/status"
	. "github.com/onsi/ginkgo/v2" //nolint:revive,stylecheck
	. "github.com/onsi/gomega"    //nolint:revive,stylecheck
	"github.com/prometheus/client_golang/prometheus"
	prometheusmodel "github.com/prometheus/client_model/go"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	policyv1 "k8s.io/api/policy/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/controllers/nodeclaim/lifecycle"
	"sigs.k8s.io/karpenter/pkg/controllers/provisioning"
	"sigs.k8s.io/karpenter/pkg/controllers/provisioning/scheduling"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/controllers/state/informer"
	"sigs.k8s.io/karpenter/pkg/metrics"
	pscheduling "sigs.k8s.io/karpenter/pkg/scheduling"
	"sigs.k8s.io/karpenter/pkg/test"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
)

const (
	ReconcilerPropagationTime = 10 * time.Second
	RequestInterval           = 1 * time.Second
)

type Bindings map[*corev1.Pod]*Binding

type Binding struct {
	NodeClaim *v1.NodeClaim
	Node      *corev1.Node
}

func (b Bindings) Get(p *corev1.Pod) *Binding {
	for k, v := range b {
		if client.ObjectKeyFromObject(k) == client.ObjectKeyFromObject(p) {
			return v
		}
	}
	return nil
}

func ExpectExists[T client.Object](ctx context.Context, c client.Client, obj T) T {
	GinkgoHelper()
	resp := reflect.New(reflect.TypeOf(*new(T)).Elem()).Interface().(T)
	Expect(c.Get(ctx, client.ObjectKeyFromObject(obj), resp)).To(Succeed())
	return resp
}

func ExpectPodExists(ctx context.Context, c client.Client, name string, namespace string) *corev1.Pod {
	GinkgoHelper()
	return ExpectExists(ctx, c, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}})
}

func ExpectNodeExists(ctx context.Context, c client.Client, name string) *corev1.Node {
	GinkgoHelper()
	return ExpectExists(ctx, c, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}})
}

func ExpectNotFound(ctx context.Context, c client.Client, objects ...client.Object) {
	GinkgoHelper()
	for _, object := range objects {
		Eventually(func() bool {
			return errors.IsNotFound(c.Get(ctx, types.NamespacedName{Name: object.GetName(), Namespace: object.GetNamespace()}, object))
		}, ReconcilerPropagationTime, RequestInterval).Should(BeTrue(), func() string {
			return fmt.Sprintf("expected %s/%s to be deleted, but it still exists", lo.Must(apiutil.GVKForObject(object, scheme.Scheme)), client.ObjectKeyFromObject(object))
		})
	}
}

func ExpectScheduled(ctx context.Context, c client.Client, pod *corev1.Pod) *corev1.Node {
	GinkgoHelper()
	p := ExpectPodExists(ctx, c, pod.Name, pod.Namespace)
	Expect(p.Spec.NodeName).ToNot(BeEmpty(), fmt.Sprintf("expected %s/%s to be scheduled", pod.Namespace, pod.Name))
	return ExpectNodeExists(ctx, c, p.Spec.NodeName)
}

func ExpectPodsScheduled(ctx context.Context, c client.Client, pods ...*corev1.Pod) {
	GinkgoHelper()
	for _, p := range pods {
		ExpectScheduled(ctx, c, p)
	}
}

func ExpectNotScheduled(ctx context.Context, c client.Client, pod *corev1.Pod) *corev1.Pod {
	GinkgoHelper()
	p := ExpectPodExists(ctx, c, pod.Name, pod.Namespace)
	Eventually(p.Spec.NodeName).Should(BeEmpty(), fmt.Sprintf("expected %s/%s to not be scheduled", pod.Namespace, pod.Name))
	return p
}

// ExpectToWait continually polls the wait group to see if there
// is a timer waiting, incrementing the clock if not.
func ExpectToWait(fakeClock *testing.FakeClock, wg *sync.WaitGroup) {
	wg.Add(1)
	Expect(fakeClock.HasWaiters()).To(BeFalse())
	go func() {
		defer GinkgoRecover()
		defer wg.Done()
		Eventually(func() bool { return fakeClock.HasWaiters() }).
			// Caution: if another go routine takes longer than this timeout to
			// wait on the clock, we will deadlock until the test suite timeout
			WithTimeout(10 * time.Second).
			WithPolling(10 * time.Millisecond).
			Should(BeTrue())
		fakeClock.Step(45 * time.Second)
	}()
}

func ExpectApplied(ctx context.Context, c client.Client, objects ...client.Object) {
	GinkgoHelper()
	for _, object := range objects {
		deletionTimestampSet := !object.GetDeletionTimestamp().IsZero()
		current := object.DeepCopyObject().(client.Object)
		statuscopy := object.DeepCopyObject().(client.Object) // Snapshot the status, since create/update may override

		// Create or Update
		if err := c.Get(ctx, client.ObjectKeyFromObject(current), current); err != nil {
			if errors.IsNotFound(err) {
				Expect(c.Create(ctx, object)).To(Succeed())
			} else {
				Expect(err).ToNot(HaveOccurred())
			}
		} else {
			object.SetResourceVersion(current.GetResourceVersion())
			Expect(c.Update(ctx, object)).To(Succeed())
		}
		// Update status
		statuscopy.SetResourceVersion(object.GetResourceVersion())
		Expect(c.Status().Update(ctx, statuscopy)).To(Or(Succeed(), MatchError("the server could not find the requested resource"))) // Some objects do not have a status

		// Re-get the object to grab the updated spec and status
		Expect(c.Get(ctx, client.ObjectKeyFromObject(object), object)).To(Succeed())

		// Set the deletion timestamp by adding a finalizer and deleting
		if deletionTimestampSet {
			ExpectDeletionTimestampSet(ctx, c, object)
		}
	}
}

func ExpectDeleted(ctx context.Context, c client.Client, objects ...client.Object) {
	GinkgoHelper()
	for _, object := range objects {
		if err := c.Delete(ctx, object, &client.DeleteOptions{GracePeriodSeconds: lo.ToPtr(int64(0))}); !errors.IsNotFound(err) {
			Expect(err).To(BeNil())
		}
		ExpectNotFound(ctx, c, object)
	}
}

func ExpectSingletonReconciled(ctx context.Context, reconciler singleton.Reconciler) reconcile.Result {
	GinkgoHelper()
	result, err := singleton.AsReconciler(reconciler).Reconcile(ctx, reconcile.Request{})
	Expect(err).ToNot(HaveOccurred())
	return result
}

func ExpectSingletonReconcileFailed(ctx context.Context, reconciler singleton.Reconciler) error {
	GinkgoHelper()
	_, err := singleton.AsReconciler(reconciler).Reconcile(ctx, reconcile.Request{})
	Expect(err).To(HaveOccurred())
	return err
}

func ExpectObjectReconciled[T client.Object](ctx context.Context, c client.Client, reconciler reconcile.ObjectReconciler[T], object T) reconcile.Result {
	GinkgoHelper()
	result, err := reconcile.AsReconciler(c, reconciler).Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(object)})
	Expect(err).ToNot(HaveOccurred())
	return result
}

func ExpectObjectReconcileFailed[T client.Object](ctx context.Context, c client.Client, reconciler reconcile.ObjectReconciler[T], object T) error {
	GinkgoHelper()
	_, err := reconcile.AsReconciler(c, reconciler).Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(object)})
	Expect(err).To(HaveOccurred())
	return err
}

// ExpectDeletionTimestampSetWithOffset ensures that the deletion timestamp is set on the objects by adding a finalizer
// and then deleting the object immediately after. This holds the object until the finalizer is patched out in the DeferCleanup
func ExpectDeletionTimestampSet(ctx context.Context, c client.Client, objects ...client.Object) {
	GinkgoHelper()
	for _, object := range objects {
		Expect(c.Get(ctx, client.ObjectKeyFromObject(object), object)).To(Succeed())
		controllerutil.AddFinalizer(object, "testing/finalizer")
		Expect(c.Update(ctx, object)).To(Succeed())
		Expect(c.Delete(ctx, object)).To(Succeed())
		DeferCleanup(func(obj client.Object) {
			mergeFrom := client.MergeFrom(obj.DeepCopyObject().(client.Object))
			obj.SetFinalizers([]string{})
			Expect(client.IgnoreNotFound(c.Patch(ctx, obj, mergeFrom))).To(Succeed())
		}, object)
	}
}

func ExpectCleanedUp(ctx context.Context, c client.Client) {
	GinkgoHelper()
	wg := sync.WaitGroup{}
	namespaces := &corev1.NamespaceList{}
	Expect(c.List(ctx, namespaces)).To(Succeed())
	ExpectFinalizersRemovedFromList(ctx, c, &corev1.NodeList{}, &v1.NodeClaimList{}, &corev1.PersistentVolumeClaimList{})
	for _, object := range []client.Object{
		&corev1.Pod{},
		&corev1.Node{},
		&appsv1.DaemonSet{},
		&nodev1.RuntimeClass{},
		&policyv1.PodDisruptionBudget{},
		&corev1.PersistentVolumeClaim{},
		&corev1.PersistentVolume{},
		&storagev1.StorageClass{},
		&v1.NodePool{},
		&v1alpha1.TestNodeClass{},
		&v1.NodeClaim{},
	} {
		for _, namespace := range namespaces.Items {
			wg.Add(1)
			go func(object client.Object, namespace string) {
				GinkgoHelper()
				defer wg.Done()
				defer GinkgoRecover()
				Expect(c.DeleteAllOf(ctx, object, client.InNamespace(namespace),
					&client.DeleteAllOfOptions{DeleteOptions: client.DeleteOptions{GracePeriodSeconds: lo.ToPtr(int64(0))}})).ToNot(HaveOccurred())
			}(object, namespace.Name)
		}
	}
	wg.Wait()
}

func ExpectFinalizersRemovedFromList(ctx context.Context, c client.Client, objectLists ...client.ObjectList) {
	GinkgoHelper()
	for _, list := range objectLists {
		Expect(c.List(ctx, list)).To(Succeed())
		Expect(meta.EachListItem(list, func(o runtime.Object) error {
			obj := o.(client.Object)
			stored := obj.DeepCopyObject().(client.Object)
			obj.SetFinalizers([]string{})
			Expect(client.IgnoreNotFound(c.Patch(ctx, obj, client.MergeFrom(stored)))).To(Succeed())
			return nil
		})).To(Succeed())
	}
}

func ExpectFinalizersRemoved(ctx context.Context, c client.Client, objs ...client.Object) {
	GinkgoHelper()
	for _, obj := range objs {
		Expect(client.IgnoreNotFound(c.Get(ctx, client.ObjectKeyFromObject(obj), obj))).To(Succeed())
		stored := obj.DeepCopyObject().(client.Object)
		obj.SetFinalizers([]string{})
		Expect(client.IgnoreNotFound(c.Patch(ctx, obj, client.MergeFrom(stored)))).To(Succeed())
	}
}

func ExpectProvisioned(ctx context.Context, c client.Client, cluster *state.Cluster, cloudProvider cloudprovider.CloudProvider, provisioner *provisioning.Provisioner, pods ...*corev1.Pod) Bindings {
	GinkgoHelper()
	bindings := ExpectProvisionedNoBinding(ctx, c, cluster, cloudProvider, provisioner, pods...)
	podKeys := sets.NewString(lo.Map(pods, func(p *corev1.Pod, _ int) string { return client.ObjectKeyFromObject(p).String() })...)
	for pod, binding := range bindings {
		// Only bind the pods that are passed through
		if podKeys.Has(client.ObjectKeyFromObject(pod).String()) {
			ExpectManualBinding(ctx, c, pod, binding.Node)
			Expect(cluster.UpdatePod(ctx, pod)).To(Succeed()) // track pod bindings
		}
	}
	return bindings
}

//nolint:gocyclo
func ExpectProvisionedNoBinding(ctx context.Context, c client.Client, cluster *state.Cluster, cloudProvider cloudprovider.CloudProvider, provisioner *provisioning.Provisioner, pods ...*corev1.Pod) Bindings {
	GinkgoHelper()
	// Persist objects
	for _, pod := range pods {
		ExpectApplied(ctx, c, pod)
	}
	// TODO: Check the error on the provisioner scheduling round
	results, err := provisioner.Schedule(ctx)
	bindings := Bindings{}
	if err != nil {
		log.Printf("error provisioning in test, %s", err)
		return bindings
	}
	for _, m := range results.NewNodeClaims {
		// TODO: Check the error on the provisioner launch
		nodeClaimName, err := provisioner.Create(ctx, m, provisioning.WithReason(metrics.ProvisionedReason))
		if err != nil {
			return bindings
		}
		nodeClaim := &v1.NodeClaim{}
		Expect(c.Get(ctx, types.NamespacedName{Name: nodeClaimName}, nodeClaim)).To(Succeed())
		nodeClaim, node := ExpectNodeClaimDeployedAndStateUpdated(ctx, c, cluster, cloudProvider, nodeClaim)
		if nodeClaim != nil && node != nil {
			for _, pod := range m.Pods {
				bindings[pod] = &Binding{
					NodeClaim: nodeClaim,
					Node:      node,
				}
			}
		}
	}
	for _, node := range results.ExistingNodes {
		for _, pod := range node.Pods {
			bindings[pod] = &Binding{
				Node: node.Node,
			}
			if node.NodeClaim != nil {
				bindings[pod].NodeClaim = node.NodeClaim
			}
		}
	}
	return bindings
}

func ExpectNodeClaimDeployedNoNode(ctx context.Context, c client.Client, cloudProvider cloudprovider.CloudProvider, nc *v1.NodeClaim) (*v1.NodeClaim, error) {
	GinkgoHelper()

	resolved, err := cloudProvider.Create(ctx, nc)
	// TODO @joinnis: Check this error rather than swallowing it. This is swallowed right now due to how we are doing some testing in the cloudprovider
	if err != nil {
		return nc, err
	}
	Expect(err).To(Succeed())

	// Make the nodeclaim ready in the status conditions
	nc = lifecycle.PopulateNodeClaimDetails(nc, resolved)
	nc.StatusConditions().SetTrue(v1.ConditionTypeLaunched)
	ExpectApplied(ctx, c, nc)
	return nc, nil
}

func ExpectNodeClaimDeployed(ctx context.Context, c client.Client, cloudProvider cloudprovider.CloudProvider, nc *v1.NodeClaim) (*v1.NodeClaim, *corev1.Node, error) {
	GinkgoHelper()

	nc, err := ExpectNodeClaimDeployedNoNode(ctx, c, cloudProvider, nc)
	if err != nil {
		return nc, nil, err
	}
	nc.StatusConditions().SetTrue(v1.ConditionTypeRegistered)

	// Mock the nodeclaim launch and node joining at the apiserver
	node := test.NodeClaimLinkedNode(nc)
	node.Spec.Taints = lo.Reject(node.Spec.Taints, func(t corev1.Taint, _ int) bool { return t.MatchTaint(&v1.UnregisteredNoExecuteTaint) })
	node.Labels = lo.Assign(node.Labels, map[string]string{v1.NodeRegisteredLabelKey: "true"})
	ExpectApplied(ctx, c, nc, node)
	return nc, node, nil
}

func ExpectNodeClaimDeployedAndStateUpdated(ctx context.Context, c client.Client, cluster *state.Cluster, cloudProvider cloudprovider.CloudProvider, nc *v1.NodeClaim) (*v1.NodeClaim, *corev1.Node) {
	GinkgoHelper()

	nc, node, err := ExpectNodeClaimDeployed(ctx, c, cloudProvider, nc)
	cluster.UpdateNodeClaim(nc)
	if err != nil {
		return nc, nil
	}
	Expect(cluster.UpdateNode(ctx, node)).To(Succeed())
	return nc, node
}

func ExpectNodeClaimsCascadeDeletion(ctx context.Context, c client.Client, nodeClaims ...*v1.NodeClaim) {
	GinkgoHelper()
	nodes := ExpectNodes(ctx, c)
	for _, nodeClaim := range nodeClaims {
		err := c.Get(ctx, client.ObjectKeyFromObject(nodeClaim), &v1.NodeClaim{})
		if !errors.IsNotFound(err) {
			continue
		}
		for _, node := range nodes {
			if node.Spec.ProviderID == nodeClaim.Status.ProviderID {
				Expect(c.Delete(ctx, node))
				ExpectFinalizersRemoved(ctx, c, node)
				ExpectNotFound(ctx, c, node)
			}
		}
	}
}

func ExpectMakeNodeClaimsInitialized(ctx context.Context, c client.Client, nodeClaims ...*v1.NodeClaim) {
	GinkgoHelper()
	for i := range nodeClaims {
		nodeClaims[i] = ExpectExists(ctx, c, nodeClaims[i])
		nodeClaims[i].StatusConditions().SetTrue(v1.ConditionTypeLaunched)
		nodeClaims[i].StatusConditions().SetTrue(v1.ConditionTypeRegistered)
		nodeClaims[i].StatusConditions().SetTrue(v1.ConditionTypeInitialized)
		ExpectApplied(ctx, c, nodeClaims[i])
	}
}

func ExpectMakeNodesInitialized(ctx context.Context, c client.Client, nodes ...*corev1.Node) {
	GinkgoHelper()
	ExpectMakeNodesReady(ctx, c, nodes...)

	for i := range nodes {
		nodes[i].Spec.Taints = lo.Reject(nodes[i].Spec.Taints, func(t corev1.Taint, _ int) bool { return t.MatchTaint(&v1.UnregisteredNoExecuteTaint) })
		nodes[i].Labels[v1.NodeRegisteredLabelKey] = "true"
		nodes[i].Labels[v1.NodeInitializedLabelKey] = "true"
		ExpectApplied(ctx, c, nodes[i])
	}
}

func ExpectMakeNodesNotReady(ctx context.Context, c client.Client, nodes ...*corev1.Node) {
	for i := range nodes {
		nodes[i] = ExpectExists(ctx, c, nodes[i])
		nodes[i].Status.Phase = corev1.NodeRunning
		nodes[i].Status.Conditions = []corev1.NodeCondition{
			{
				Type:               corev1.NodeReady,
				Status:             corev1.ConditionFalse,
				LastHeartbeatTime:  metav1.Now(),
				LastTransitionTime: metav1.Now(),
				Reason:             "NotReady",
			},
		}
		if nodes[i].Labels == nil {
			nodes[i].Labels = map[string]string{}
		}
		ExpectApplied(ctx, c, nodes[i])
	}
}

func ExpectMakeNodesReady(ctx context.Context, c client.Client, nodes ...*corev1.Node) {
	for i := range nodes {
		nodes[i] = ExpectExists(ctx, c, nodes[i])
		nodes[i].Status.Phase = corev1.NodeRunning
		nodes[i].Status.Conditions = []corev1.NodeCondition{
			{
				Type:               corev1.NodeReady,
				Status:             corev1.ConditionTrue,
				LastHeartbeatTime:  metav1.Now(),
				LastTransitionTime: metav1.Now(),
				Reason:             "KubeletReady",
			},
		}
		if nodes[i].Labels == nil {
			nodes[i].Labels = map[string]string{}
		}
		// Remove any of the known ephemeral taints to make the Node ready
		nodes[i].Spec.Taints = lo.Reject(nodes[i].Spec.Taints, func(taint corev1.Taint, _ int) bool {
			_, found := lo.Find(pscheduling.KnownEphemeralTaints, func(t corev1.Taint) bool {
				return t.MatchTaint(&taint)
			})
			return found
		})
		ExpectApplied(ctx, c, nodes[i])
	}
}

func ExpectReconcileSucceeded(ctx context.Context, reconciler reconcile.Reconciler, key client.ObjectKey) reconcile.Result {
	GinkgoHelper()
	result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
	Expect(err).ToNot(HaveOccurred())
	return result
}

func ExpectStatusConditionExists(obj status.Object, t string) status.Condition {
	GinkgoHelper()
	conds := obj.GetConditions()
	cond, ok := lo.Find(conds, func(c status.Condition) bool {
		return c.Type == t
	})
	Expect(ok).To(BeTrue())
	return cond
}

func ExpectOwnerReferenceExists(obj, owner client.Object) metav1.OwnerReference {
	or, found := lo.Find(obj.GetOwnerReferences(), func(o metav1.OwnerReference) bool {
		return o.UID == owner.GetUID()
	})
	Expect(found).To(BeTrue())
	return or
}

// ExpectMetricName attempts to resolve a metric name from a collector. This function will work so long as the fully
// qualified name is a single metric name. This holds true for the built in types, but may not for custom collectors.
func ExpectMetricName(collector prometheus.Collector) string {
	GinkgoHelper()

	// Prometheus defines an async method to resolve the description for a collector. This is simpler than it looks,
	// Describe just returns a string through the provided channel.
	result := make(chan *prometheus.Desc)
	var desc *prometheus.Desc
	go func() {
		collector.Describe(result)
	}()
	select {
	case desc = <-result:
	// Add a timeout so a failure doesn't result in stalling the entire test suite. This should never occur.
	case <-time.After(time.Second):
	}
	Expect(desc).ToNot(BeNil())

	// Extract the fully qualified name from the description string. This is just different enough from json that we
	// need to parse with regex.
	rgx := regexp.MustCompile(`^.*fqName:\s*"([^"]*).*$`)
	matches := rgx.FindStringSubmatch(desc.String())
	Expect(len(matches)).To(Equal(2))
	return matches[1]
}

// FindMetricWithLabelValues attempts to find a metric with a name with a set of label values
// If no metric is found, the *prometheusmodel.Metric will be nil
func FindMetricWithLabelValues(name string, labelValues map[string]string) (*prometheusmodel.Metric, bool) {
	GinkgoHelper()
	metrics, err := crmetrics.Registry.Gather()
	Expect(err).To(BeNil())

	mf, found := lo.Find(metrics, func(mf *prometheusmodel.MetricFamily) bool {
		return mf.GetName() == name
	})
	if !found {
		return nil, false
	}
	for _, m := range mf.Metric {
		temp := lo.Assign(labelValues)
		for _, labelPair := range m.Label {
			if v, ok := temp[labelPair.GetName()]; ok && v == labelPair.GetValue() {
				delete(temp, labelPair.GetName())
			}
		}
		if len(temp) == 0 {
			return m, true
		}
	}
	return nil, false
}

func ExpectMetricGaugeValue(collector opmetrics.GaugeMetric, expectedValue float64, labels map[string]string) {
	GinkgoHelper()
	metricName := ExpectMetricName(collector.(*opmetrics.PrometheusGauge))
	metric, ok := FindMetricWithLabelValues(metricName, labels)
	Expect(ok).To(BeTrue(), "Metric "+metricName+" should be available")
	Expect(lo.FromPtr(metric.Gauge.Value)).To(Equal(expectedValue), "Metric "+metricName+" should have the expected value")
}

func ExpectMetricCounterValue(collector opmetrics.CounterMetric, expectedValue float64, labels map[string]string) {
	GinkgoHelper()
	metricName := ExpectMetricName(collector.(*opmetrics.PrometheusCounter))
	metric, ok := FindMetricWithLabelValues(metricName, labels)
	Expect(ok).To(BeTrue(), "Metric "+metricName+" should be available")
	Expect(lo.FromPtr(metric.Counter.Value)).To(Equal(expectedValue), "Metric "+metricName+" should have the expected value")
}

func ExpectMetricHistogramSampleCountValue(metricName string, expectedValue uint64, labels map[string]string) {
	GinkgoHelper()
	metric, ok := FindMetricWithLabelValues(metricName, labels)
	Expect(ok).To(BeTrue(), "Metric "+metricName+" should be available")
	Expect(lo.FromPtr(metric.Histogram.SampleCount)).To(Equal(expectedValue), "Metric "+metricName+" should have the expected value")
}

func ExpectManualBinding(ctx context.Context, c client.Client, pod *corev1.Pod, node *corev1.Node) {
	GinkgoHelper()
	Expect(c.Create(ctx, &corev1.Binding{
		TypeMeta: pod.TypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.ObjectMeta.Name,
			Namespace: pod.ObjectMeta.Namespace,
			UID:       pod.ObjectMeta.UID,
		},
		Target: corev1.ObjectReference{
			Name: node.Name,
		},
	})).To(Succeed())
	Eventually(func(g Gomega) {
		g.Expect(c.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
		g.Expect(pod.Spec.NodeName).To(Equal(node.Name))
	}).Should(Succeed())
}

func ExpectSkew(ctx context.Context, c client.Client, namespace string, constraint *corev1.TopologySpreadConstraint) Assertion {
	GinkgoHelper()
	nodes := &corev1.NodeList{}
	Expect(c.List(ctx, nodes)).To(Succeed())
	pods := &corev1.PodList{}
	Expect(c.List(ctx, pods, scheduling.TopologyListOptions(namespace, constraint.LabelSelector))).To(Succeed())
	skew := map[string]int{}
	for i, pod := range pods.Items {
		if scheduling.IgnoredForTopology(&pods.Items[i]) {
			continue
		}
		for _, node := range nodes.Items {
			if pod.Spec.NodeName == node.Name {
				switch constraint.TopologyKey {
				case corev1.LabelHostname:
					skew[node.Name]++ // Check node name since hostname labels aren't applied
				default:
					if key, ok := node.Labels[constraint.TopologyKey]; ok {
						skew[key]++
					}
				}
			}
		}
	}
	return Expect(skew)
}

// ExpectResources expects all the resources in expected to exist in real with the same values
func ExpectResources(expected, real corev1.ResourceList) {
	GinkgoHelper()
	for k, v := range expected {
		realV := real[k]
		Expect(v.Value()).To(BeNumerically("~", realV.Value()))
	}
}

func ExpectNodes(ctx context.Context, c client.Client) []*corev1.Node {
	GinkgoHelper()
	nodeList := &corev1.NodeList{}
	Expect(c.List(ctx, nodeList)).To(Succeed())
	return lo.ToSlicePtr(nodeList.Items)
}

func ExpectNodeClaims(ctx context.Context, c client.Client) []*v1.NodeClaim {
	GinkgoHelper()
	nodeClaims := &v1.NodeClaimList{}
	Expect(c.List(ctx, nodeClaims)).To(Succeed())
	return lo.ToSlicePtr(nodeClaims.Items)
}

func ExpectStateNodeExists(cluster *state.Cluster, node *corev1.Node) *state.StateNode {
	GinkgoHelper()
	var ret *state.StateNode
	cluster.ForEachNode(func(n *state.StateNode) bool {
		if n.Node.Name != node.Name {
			return true
		}
		ret = n.DeepCopy()
		return false
	})
	Expect(ret).ToNot(BeNil())
	return ret
}

func ExpectStateNodeExistsForNodeClaim(cluster *state.Cluster, nodeClaim *v1.NodeClaim) *state.StateNode {
	GinkgoHelper()
	var ret *state.StateNode
	cluster.ForEachNode(func(n *state.StateNode) bool {
		if n.NodeClaim.Status.ProviderID != nodeClaim.Status.ProviderID {
			return true
		}
		ret = n.DeepCopy()
		return false
	})
	Expect(ret).ToNot(BeNil())
	return ret
}

func ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx context.Context, c client.Client, nodeStateController *informer.NodeController, nodeClaimStateController *informer.NodeClaimController, nodes []*corev1.Node, nodeClaims []*v1.NodeClaim) {
	GinkgoHelper()

	ExpectMakeNodesInitialized(ctx, c, nodes...)
	ExpectMakeNodeClaimsInitialized(ctx, c, nodeClaims...)

	// Inform cluster state about node and nodeclaim readiness
	for _, n := range nodes {
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(n))
	}
	for _, m := range nodeClaims {
		ExpectReconcileSucceeded(ctx, nodeClaimStateController, client.ObjectKeyFromObject(m))
	}
}

// ExpectEvicted triggers an eviction call for all the passed pods
func ExpectEvicted(ctx context.Context, c client.Client, pods ...*corev1.Pod) {
	GinkgoHelper()

	for _, pod := range pods {
		Expect(c.SubResource("eviction").Create(ctx, pod, &policyv1.Eviction{})).To(Succeed())
	}
	EventuallyExpectTerminating(ctx, c, lo.Map(pods, func(p *corev1.Pod, _ int) client.Object { return p })...)
}

// EventuallyExpectTerminating ensures that the deletion timestamp is eventually set
// We need this since there is some propagation time for the eviction API to set the deletionTimestamp
func EventuallyExpectTerminating(ctx context.Context, c client.Client, objs ...client.Object) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		for _, obj := range objs {
			g.Expect(c.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
			g.Expect(obj.GetDeletionTimestamp().IsZero()).ToNot(BeTrue())
		}
	}, time.Second).Should(Succeed())
}

// ConsistentlyExpectNotTerminating ensures that the deletion timestamp is not set
// We need this since there is some propagation time for the eviction API to set the deletionTimestamp
func ConsistentlyExpectNotTerminating(ctx context.Context, c client.Client, objs ...client.Object) {
	GinkgoHelper()

	Consistently(func(g Gomega) {
		for _, obj := range objs {
			g.Expect(c.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
			g.Expect(obj.GetDeletionTimestamp().IsZero()).To(BeTrue())
		}
	}, time.Second).Should(Succeed())
}
