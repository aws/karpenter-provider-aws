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

package common

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/transport"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	pscheduling "sigs.k8s.io/karpenter/pkg/controllers/provisioning/scheduling"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	"sigs.k8s.io/karpenter/pkg/test"
	coreresources "sigs.k8s.io/karpenter/pkg/utils/resources"
)

func (env *Environment) ExpectCreated(objects ...client.Object) {
	GinkgoHelper()
	for _, object := range objects {
		Eventually(func(g Gomega) {
			object.SetLabels(lo.Assign(object.GetLabels(), map[string]string{
				test.DiscoveryLabel: "unspecified",
			}))
			g.Expect(env.Client.Create(env, object)).To(Succeed())
		}).WithTimeout(time.Second * 10).Should(Succeed())
	}
}

func (env *Environment) ExpectDeleted(objects ...client.Object) {
	GinkgoHelper()
	for _, object := range objects {
		Eventually(func(g Gomega) {
			g.Expect(client.IgnoreNotFound(env.Client.Delete(env, object, client.PropagationPolicy(metav1.DeletePropagationForeground), &client.DeleteOptions{GracePeriodSeconds: lo.ToPtr(int64(0))}))).To(Succeed())
		}).WithTimeout(time.Second * 10).Should(Succeed())
	}
}

// ExpectUpdated will update objects in the cluster to match the inputs.
// WARNING: This ignores the resource version check, which can result in
// overwriting changes made by other controllers in the cluster.
// This is useful in ensuring that we can clean up resources by patching
// out finalizers.
// Grab the object before making the updates to reduce the chance of this race.
func (env *Environment) ExpectUpdated(objects ...client.Object) {
	GinkgoHelper()
	for _, o := range objects {
		Eventually(func(g Gomega) {
			current := o.DeepCopyObject().(client.Object)
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(current), current)).To(Succeed())
			if current.GetResourceVersion() != o.GetResourceVersion() {
				log.FromContext(env).Info(fmt.Sprintf("detected an update to an object (%s) with an outdated resource version, did you get the latest version of the object before patching?", lo.Must(apiutil.GVKForObject(o, env.Client.Scheme()))))
			}
			o.SetResourceVersion(current.GetResourceVersion())
			g.Expect(env.Client.Update(env.Context, o)).To(Succeed())
		}).WithTimeout(time.Second * 10).Should(Succeed())
	}
}

// ExpectStatusUpdated will update objects in the cluster to match the inputs.
// WARNING: This ignores the resource version check, which can result in
// overwriting changes made by other controllers in the cluster.
// This is useful in ensuring that we can clean up resources by patching
// out finalizers.
// Grab the object before making the updates to reduce the chance of this race.
func (env *Environment) ExpectStatusUpdated(objects ...client.Object) {
	GinkgoHelper()
	for _, o := range objects {
		Eventually(func(g Gomega) {
			current := o.DeepCopyObject().(client.Object)
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(current), current)).To(Succeed())
			if current.GetResourceVersion() != o.GetResourceVersion() {
				log.FromContext(env).Info(fmt.Sprintf("detected an update to an object (%s) with an outdated resource version, did you get the latest version of the object before patching?", lo.Must(apiutil.GVKForObject(o, env.Client.Scheme()))))
			}
			o.SetResourceVersion(current.GetResourceVersion())
			g.Expect(env.Client.Status().Update(env.Context, o)).To(Succeed())
		}).WithTimeout(time.Second * 10).Should(Succeed())
	}
}

func ReplaceNodeConditions(node *corev1.Node, conds ...corev1.NodeCondition) *corev1.Node {
	keys := sets.New[string](lo.Map(conds, func(c corev1.NodeCondition, _ int) string { return string(c.Type) })...)
	node.Status.Conditions = lo.Reject(node.Status.Conditions, func(c corev1.NodeCondition, _ int) bool {
		return keys.Has(string(c.Type))
	})
	node.Status.Conditions = append(node.Status.Conditions, conds...)
	return node
}

// ExpectCreatedOrUpdated can update objects in the cluster to match the inputs.
// WARNING: ExpectUpdated ignores the resource version check, which can result in
// overwriting changes made by other controllers in the cluster.
// This is useful in ensuring that we can clean up resources by patching
// out finalizers.
// Grab the object before making the updates to reduce the chance of this race.
func (env *Environment) ExpectCreatedOrUpdated(objects ...client.Object) {
	GinkgoHelper()
	for _, o := range objects {
		current := o.DeepCopyObject().(client.Object)
		err := env.Client.Get(env, client.ObjectKeyFromObject(current), current)
		if err != nil {
			if errors.IsNotFound(err) {
				env.ExpectCreated(o)
			} else {
				Fail(fmt.Sprintf("Getting object %s, %v", client.ObjectKeyFromObject(o), err))
			}
		} else {
			env.ExpectUpdated(o)
		}
	}
}

func (env *Environment) ExpectSettings() (res []corev1.EnvVar) {
	GinkgoHelper()

	d := &appsv1.Deployment{}
	Expect(env.Client.Get(env.Context, types.NamespacedName{Namespace: "kube-system", Name: "karpenter"}, d)).To(Succeed())
	Expect(d.Spec.Template.Spec.Containers).To(HaveLen(1))
	return lo.Map(d.Spec.Template.Spec.Containers[0].Env, func(v corev1.EnvVar, _ int) corev1.EnvVar {
		return *v.DeepCopy()
	})
}

func (env *Environment) ExpectSettingsReplaced(vars ...corev1.EnvVar) {
	GinkgoHelper()

	d := &appsv1.Deployment{}
	Expect(env.Client.Get(env.Context, types.NamespacedName{Namespace: "kube-system", Name: "karpenter"}, d)).To(Succeed())
	Expect(d.Spec.Template.Spec.Containers).To(HaveLen(1))

	stored := d.DeepCopy()
	d.Spec.Template.Spec.Containers[0].Env = vars

	if !equality.Semantic.DeepEqual(d, stored) {
		By("replacing environment variables for karpenter deployment")
		Expect(env.Client.Patch(env.Context, d, client.StrategicMergeFrom(stored))).To(Succeed())
		env.EventuallyExpectKarpenterRestarted()
	}
}

func (env *Environment) ExpectSettingsOverridden(vars ...corev1.EnvVar) {
	GinkgoHelper()

	d := &appsv1.Deployment{}
	Expect(env.Client.Get(env.Context, types.NamespacedName{Namespace: "kube-system", Name: "karpenter"}, d)).To(Succeed())
	Expect(d.Spec.Template.Spec.Containers).To(HaveLen(1))

	stored := d.DeepCopy()
	for _, v := range vars {
		if _, i, ok := lo.FindIndexOf(d.Spec.Template.Spec.Containers[0].Env, func(e corev1.EnvVar) bool {
			return e.Name == v.Name
		}); ok {
			d.Spec.Template.Spec.Containers[0].Env[i] = v
		} else {
			d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env, v)
		}
	}
	if !equality.Semantic.DeepEqual(d, stored) {
		By("overriding environment variables for karpenter deployment")
		Expect(env.Client.Patch(env.Context, d, client.StrategicMergeFrom(stored))).To(Succeed())
		env.EventuallyExpectKarpenterRestarted()
	}
}

func (env *Environment) ExpectSettingsRemoved(vars ...corev1.EnvVar) {
	GinkgoHelper()

	varNames := sets.New(lo.Map(vars, func(v corev1.EnvVar, _ int) string { return v.Name })...)

	d := &appsv1.Deployment{}
	Expect(env.Client.Get(env.Context, types.NamespacedName{Namespace: "kube-system", Name: "karpenter"}, d)).To(Succeed())
	Expect(d.Spec.Template.Spec.Containers).To(HaveLen(1))

	stored := d.DeepCopy()
	d.Spec.Template.Spec.Containers[0].Env = lo.Reject(d.Spec.Template.Spec.Containers[0].Env, func(v corev1.EnvVar, _ int) bool {
		return varNames.Has(v.Name)
	})
	if !equality.Semantic.DeepEqual(d, stored) {
		By("removing environment variables for karpenter deployment")
		Expect(env.Client.Patch(env.Context, d, client.StrategicMergeFrom(stored))).To(Succeed())
		env.EventuallyExpectKarpenterRestarted()
	}
}

func (env *Environment) ExpectConfigMapExists(key types.NamespacedName) *corev1.ConfigMap {
	GinkgoHelper()
	cm := &corev1.ConfigMap{}
	Expect(env.Client.Get(env, key, cm)).To(Succeed())
	return cm
}

func (env *Environment) ExpectConfigMapDataReplaced(key types.NamespacedName, data ...map[string]string) (changed bool) {
	GinkgoHelper()
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
	}
	err := env.Client.Get(env, key, cm)
	Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())

	stored := cm.DeepCopy()
	cm.Data = lo.Assign(data...) // Completely replace the data

	// If the data hasn't changed, we can just return and not update anything
	if equality.Semantic.DeepEqual(stored, cm) {
		return false
	}
	// Update the configMap to update the settings
	env.ExpectCreatedOrUpdated(cm)
	return true
}

func (env *Environment) ExpectConfigMapDataOverridden(key types.NamespacedName, data ...map[string]string) (changed bool) {
	GinkgoHelper()
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
	}
	err := env.Client.Get(env, key, cm)
	Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())

	cm.Data = lo.Assign(append([]map[string]string{cm.Data}, data...)...)

	// Update the configMap to update the settings
	env.ExpectCreatedOrUpdated(cm)
	return true
}

func (env *Environment) ExpectPodENIEnabled() {
	GinkgoHelper()
	env.ExpectDaemonSetEnvironmentVariableUpdated(types.NamespacedName{Namespace: "kube-system", Name: "aws-node"},
		"ENABLE_POD_ENI", "true", "aws-node")
}

func (env *Environment) ExpectPodENIDisabled() {
	GinkgoHelper()
	env.ExpectDaemonSetEnvironmentVariableUpdated(types.NamespacedName{Namespace: "kube-system", Name: "aws-node"},
		"ENABLE_POD_ENI", "false", "aws-node")
}

func (env *Environment) ExpectPrefixDelegationEnabled() {
	GinkgoHelper()
	env.ExpectDaemonSetEnvironmentVariableUpdated(types.NamespacedName{Namespace: "kube-system", Name: "aws-node"},
		"ENABLE_PREFIX_DELEGATION", "true", "aws-node")
}

func (env *Environment) ExpectPrefixDelegationDisabled() {
	GinkgoHelper()
	env.ExpectDaemonSetEnvironmentVariableUpdated(types.NamespacedName{Namespace: "kube-system", Name: "aws-node"},
		"ENABLE_PREFIX_DELEGATION", "false", "aws-node")
}

func (env *Environment) ExpectExists(obj client.Object) client.Object {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
	}).WithTimeout(time.Second * 5).Should(Succeed())
	return obj
}

func (env *Environment) EventuallyExpectBound(pods ...*corev1.Pod) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		for _, pod := range pods {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
			g.Expect(pod.Spec.NodeName).ToNot(BeEmpty())
		}
	}).Should(Succeed())
}

func (env *Environment) EventuallyExpectHealthy(pods ...*corev1.Pod) {
	GinkgoHelper()
	env.EventuallyExpectHealthyWithTimeout(-1, pods...)
}

func (env *Environment) EventuallyExpectTerminating(pods ...*corev1.Pod) {
	GinkgoHelper()
	env.EventuallyExpectTerminatingWithTimeout(-1, pods...)
}

func (env *Environment) EventuallyExpectTerminatingWithTimeout(timeout time.Duration, pods ...*corev1.Pod) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		for _, pod := range pods {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
			g.Expect(pod.DeletionTimestamp.IsZero()).To(BeFalse())
		}
	}).WithTimeout(timeout).Should(Succeed())
}

func (env *Environment) EventuallyExpectNoLeakedKubeNodeLease() {
	GinkgoHelper()
	// expect no kube node lease to be leaked
	leases := &coordinationv1.LeaseList{}
	Expect(env.Client.List(env.Context, leases, client.InNamespace("kube-node-lease"))).To(Succeed())
	leakedLeases := lo.Filter(leases.Items, func(l coordinationv1.Lease, _ int) bool {
		return l.OwnerReferences == nil
	})
	Expect(leakedLeases).To(HaveLen(0))
}

func (env *Environment) EventuallyExpectHealthyWithTimeout(timeout time.Duration, pods ...*corev1.Pod) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		for _, pod := range pods {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
			g.Expect(pod.Status.Conditions).To(ContainElement(And(
				HaveField("Type", Equal(corev1.PodReady)),
				HaveField("Status", Equal(corev1.ConditionTrue)),
			)))
		}
	}).WithTimeout(timeout).Should(Succeed())
}

func (env *Environment) ConsistentlyExpectTerminatingPods(duration time.Duration, pods ...*corev1.Pod) {
	GinkgoHelper()
	By(fmt.Sprintf("expecting %d pods to be terminating for %s", len(pods), duration))
	Consistently(func(g Gomega) {
		for _, pod := range pods {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
			g.Expect(pod.DeletionTimestamp.IsZero()).To(BeFalse())
		}
	}, duration.String()).Should(Succeed())
}

func (env *Environment) ConsistentlyExpectActivePods(duration time.Duration, pods ...*corev1.Pod) {
	GinkgoHelper()
	By(fmt.Sprintf("expecting %d pods to be live for %s", len(pods), duration))
	Consistently(func(g Gomega) {
		for _, pod := range pods {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
			g.Expect(pod.DeletionTimestamp.IsZero()).To(BeTrue())
		}
	}, duration.String()).Should(Succeed())
}

func (env *Environment) ConsistentlyExpectHealthyPods(duration time.Duration, pods ...*corev1.Pod) {
	GinkgoHelper()
	By(fmt.Sprintf("expecting %d pods to be ready for %s", len(pods), duration))
	Consistently(func(g Gomega) {
		for _, pod := range pods {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
			g.Expect(pod.Status.Conditions).To(ContainElement(And(
				HaveField("Type", Equal(corev1.PodReady)),
				HaveField("Status", Equal(corev1.ConditionTrue)),
			)))
		}
	}, duration.String()).Should(Succeed())
}

func (env *Environment) EventuallyExpectKarpenterRestarted() {
	GinkgoHelper()
	By("rolling out the new karpenter deployment")
	env.EventuallyExpectRollout("karpenter", "kube-system")
	env.ExpectKarpenterLeaseOwnerChanged()
}

func (env *Environment) ExpectKarpenterLeaseOwnerChanged() {
	GinkgoHelper()

	By("waiting for a new karpenter pod to hold the lease")
	pods := env.ExpectKarpenterPods()
	Eventually(func(g Gomega) {
		name := env.ExpectActiveKarpenterPodName()
		g.Expect(lo.ContainsBy(pods, func(p *corev1.Pod) bool {
			return p.Name == name
		})).To(BeTrue())
	}).Should(Succeed())
}

func (env *Environment) EventuallyExpectRollout(name, namespace string) {
	GinkgoHelper()
	By("restarting the deployment")
	deploy := &appsv1.Deployment{}
	Expect(env.Client.Get(env.Context, types.NamespacedName{Name: name, Namespace: namespace}, deploy)).To(Succeed())

	stored := deploy.DeepCopy()
	restartedAtAnnotation := map[string]string{
		"kubectl.kubernetes.io/restartedAt": time.Now().Format(time.RFC3339),
	}
	deploy.Spec.Template.Annotations = lo.Assign(deploy.Spec.Template.Annotations, restartedAtAnnotation)
	Expect(env.Client.Patch(env.Context, deploy, client.StrategicMergeFrom(stored))).To(Succeed())

	By("waiting for the newly generated deployment to rollout")
	Eventually(func(g Gomega) {
		podList := &corev1.PodList{}
		g.Expect(env.Client.List(env.Context, podList, client.InNamespace(namespace))).To(Succeed())
		pods := lo.Filter(podList.Items, func(p corev1.Pod, _ int) bool {
			return p.Annotations["kubectl.kubernetes.io/restartedAt"] == restartedAtAnnotation["kubectl.kubernetes.io/restartedAt"]
		})
		g.Expect(len(pods)).To(BeNumerically("==", lo.FromPtr(deploy.Spec.Replicas)))
		for _, pod := range pods {
			g.Expect(pod.Status.Conditions).To(ContainElement(And(
				HaveField("Type", Equal(corev1.PodReady)),
				HaveField("Status", Equal(corev1.ConditionTrue)),
			)))
			g.Expect(pod.Status.Phase).To(Equal(corev1.PodRunning))
		}
	}).Should(Succeed())
}

func (env *Environment) ExpectKarpenterPods() []*corev1.Pod {
	GinkgoHelper()
	podList := &corev1.PodList{}
	Expect(env.Client.List(env.Context, podList, client.MatchingLabels{
		"app.kubernetes.io/instance": "karpenter",
	})).To(Succeed())
	return lo.Map(podList.Items, func(p corev1.Pod, _ int) *corev1.Pod { return &p })
}

func (env *Environment) ExpectActiveKarpenterPodName() string {
	GinkgoHelper()
	lease := &coordinationv1.Lease{}
	Expect(env.Client.Get(env.Context, types.NamespacedName{Name: "karpenter-leader-election", Namespace: "kube-system"}, lease)).To(Succeed())

	// Holder identity for lease is always in the format "<pod-name>_<pseudo-random-value>
	holderArr := strings.Split(lo.FromPtr(lease.Spec.HolderIdentity), "_")
	Expect(len(holderArr)).To(BeNumerically(">", 0))

	return holderArr[0]
}

func (env *Environment) ExpectActiveKarpenterPod() *corev1.Pod {
	GinkgoHelper()
	podName := env.ExpectActiveKarpenterPodName()

	pod := &corev1.Pod{}
	Expect(env.Client.Get(env.Context, types.NamespacedName{Name: podName, Namespace: "kube-system"}, pod)).To(Succeed())
	return pod
}

func (env *Environment) EventuallyExpectPendingPodCount(selector labels.Selector, numPods int) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		g.Expect(env.Monitor.PendingPodsCount(selector)).To(Equal(numPods))
	}).Should(Succeed())
}

func (env *Environment) EventuallyExpectBoundPodCount(selector labels.Selector, numPods int) []*corev1.Pod {
	GinkgoHelper()
	var res []*corev1.Pod
	Eventually(func(g Gomega) {
		res = []*corev1.Pod{}
		podList := &corev1.PodList{}
		g.Expect(env.Client.List(env.Context, podList, client.MatchingLabelsSelector{Selector: selector})).To(Succeed())
		for i := range podList.Items {
			if podList.Items[i].Spec.NodeName != "" {
				res = append(res, &podList.Items[i])
			}
		}
		g.Expect(res).To(HaveLen(numPods))
	}).Should(Succeed())
	return res
}

func (env *Environment) EventuallyExpectHealthyPodCount(selector labels.Selector, numPods int) []*corev1.Pod {
	By(fmt.Sprintf("waiting for %d pods matching selector %s to be ready", numPods, selector.String()))
	GinkgoHelper()
	return env.EventuallyExpectHealthyPodCountWithTimeout(-1, selector, numPods)
}

func (env *Environment) EventuallyExpectHealthyPodCountWithTimeout(timeout time.Duration, selector labels.Selector, numPods int) []*corev1.Pod {
	GinkgoHelper()
	var pods []*corev1.Pod
	Eventually(func(g Gomega) {
		pods = env.Monitor.RunningPods(selector)
		g.Expect(pods).To(HaveLen(numPods))
	}).WithTimeout(timeout).Should(Succeed())
	return pods
}

func (env *Environment) ExpectPodsMatchingSelector(selector labels.Selector) []*corev1.Pod {
	GinkgoHelper()

	podList := &corev1.PodList{}
	Expect(env.Client.List(env.Context, podList, client.MatchingLabelsSelector{Selector: selector})).To(Succeed())
	return lo.ToSlicePtr(podList.Items)
}

func (env *Environment) EventuallyExpectUniqueNodeNames(selector labels.Selector, uniqueNames int) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		pods := env.Monitor.RunningPods(selector)
		nodeNames := sets.NewString()
		for _, pod := range pods {
			nodeNames.Insert(pod.Spec.NodeName)
		}
		g.Expect(len(nodeNames)).To(BeNumerically("==", uniqueNames))
	}).Should(Succeed())
}

func (env *Environment) eventuallyExpectScaleDown() {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		// expect the current node count to be what it was when the test started
		g.Expect(env.Monitor.NodeCount()).To(Equal(env.StartingNodeCount))
	}).Should(Succeed(), fmt.Sprintf("expected scale down to %d nodes, had %d", env.StartingNodeCount, env.Monitor.NodeCount()))
}

func (env *Environment) EventuallyExpectNotFound(objects ...client.Object) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		for _, object := range objects {
			err := env.Client.Get(env, client.ObjectKeyFromObject(object), object)
			g.Expect(errors.IsNotFound(err)).To(BeTrue())
		}
	}).Should(Succeed())
}

func (env *Environment) ExpectCreatedNodeCount(comparator string, count int) []*corev1.Node {
	GinkgoHelper()
	createdNodes := env.Monitor.CreatedNodes()
	Expect(len(createdNodes)).To(BeNumerically(comparator, count),
		fmt.Sprintf("expected %d created nodes, had %d (%v)", count, len(createdNodes), NodeNames(createdNodes)))
	return createdNodes
}

func (env *Environment) ExpectNodeCount(comparator string, count int) []*corev1.Node {
	GinkgoHelper()

	nodeList := &corev1.NodeList{}
	Expect(env.Client.List(env, nodeList, client.HasLabels{test.DiscoveryLabel})).To(Succeed())
	Expect(len(nodeList.Items)).To(BeNumerically(comparator, count))
	return lo.ToSlicePtr(nodeList.Items)
}

func (env *Environment) ExpectNodeClaimCount(comparator string, count int) []*karpv1.NodeClaim {
	GinkgoHelper()

	nodeClaimList := &karpv1.NodeClaimList{}
	Expect(env.Client.List(env, nodeClaimList, client.HasLabels{test.DiscoveryLabel})).To(Succeed())
	Expect(len(nodeClaimList.Items)).To(BeNumerically(comparator, count))
	return lo.ToSlicePtr(nodeClaimList.Items)
}

func NodeClaimNames(nodeClaims []*karpv1.NodeClaim) []string {
	return lo.Map(nodeClaims, func(n *karpv1.NodeClaim, index int) string {
		return n.Name
	})
}

func NodeNames(nodes []*corev1.Node) []string {
	return lo.Map(nodes, func(n *corev1.Node, index int) string {
		return n.Name
	})
}

func (env *Environment) ConsistentlyExpectNodeCount(comparator string, count int, duration time.Duration) []*corev1.Node {
	GinkgoHelper()
	By(fmt.Sprintf("expecting nodes to be %s to %d for %s", comparator, count, duration))
	nodeList := &corev1.NodeList{}
	Consistently(func(g Gomega) {
		g.Expect(env.Client.List(env, nodeList, client.HasLabels{test.DiscoveryLabel})).To(Succeed())
		g.Expect(len(nodeList.Items)).To(BeNumerically(comparator, count),
			fmt.Sprintf("expected %d nodes, had %d (%v) for %s", count, len(nodeList.Items), NodeNames(lo.ToSlicePtr(nodeList.Items)), duration))
	}, duration.String()).Should(Succeed())
	return lo.ToSlicePtr(nodeList.Items)
}

// ConsistentlyExpectNoDisruptions asserts that the number of tainted nodes remains the same.
// And that the number of nodeclaims remains the same.
func (env *Environment) ConsistentlyExpectNoDisruptions(nodeCount int, duration time.Duration) {
	GinkgoHelper()
	Consistently(func(g Gomega) {
		nodeClaimList := &karpv1.NodeClaimList{}
		g.Expect(env.Client.List(env, nodeClaimList, client.HasLabels{test.DiscoveryLabel})).To(Succeed())
		g.Expect(nodeClaimList.Items).To(HaveLen(nodeCount))
		nodeList := &corev1.NodeList{}
		g.Expect(env.Client.List(env, nodeList, client.HasLabels{test.DiscoveryLabel})).To(Succeed())
		g.Expect(nodeList.Items).To(HaveLen(nodeCount))
		nodeList.Items = lo.Filter(nodeList.Items, func(n corev1.Node, _ int) bool {
			_, ok := lo.Find(n.Spec.Taints, func(t corev1.Taint) bool {
				return t.MatchTaint(&karpv1.DisruptedNoScheduleTaint)
			})
			return ok
		})
		g.Expect(nodeList.Items).To(HaveLen(0))
	}, duration).Should(Succeed())
}

// ConsistentlyExpectDisruptionsUntilNoneLeft consistently ensures a max on number of concurrently disrupting and non-terminating nodes.
// This actually uses an Eventually() under the hood so that when we reach 0 tainted nodes we exit early.
// We use the StopTrying() so that we can exit the Eventually() if we've breached an assertion on total concurrency of disruptions.
// For example: if we have 5 nodes, with a budget of 2 nodes, we ensure that `disruptingNodes <= maxNodesDisrupting=2`
// We use nodesAtStart+maxNodesDisrupting to assert that we're not creating too many instances in replacement.
func (env *Environment) ConsistentlyExpectDisruptionsUntilNoneLeft(nodesAtStart, maxNodesDisrupting int, timeout time.Duration) {
	GinkgoHelper()
	nodes := []corev1.Node{}
	// We use an eventually to exit when we detect the number of tainted/disrupted nodes matches our target.
	Eventually(func(g Gomega) {
		// Grab Nodes and NodeClaims
		nodeClaimList := &karpv1.NodeClaimList{}
		nodeList := &corev1.NodeList{}
		g.Expect(env.Client.List(env, nodeClaimList, client.HasLabels{test.DiscoveryLabel})).To(Succeed())
		g.Expect(env.Client.List(env, nodeList, client.HasLabels{test.DiscoveryLabel})).To(Succeed())

		// Don't include NodeClaims with the `Terminating` status condition, as they're not included in budgets
		removedProviderIDs := sets.Set[string]{}
		nodeClaimList.Items = lo.Filter(nodeClaimList.Items, func(nc karpv1.NodeClaim, _ int) bool {
			if !nc.StatusConditions().IsTrue(karpv1.ConditionTypeInstanceTerminating) {
				return true
			}
			removedProviderIDs.Insert(nc.Status.ProviderID)
			return false
		})
		if len(nodeClaimList.Items) > nodesAtStart+maxNodesDisrupting {
			StopTrying(fmt.Sprintf("Too many nodeclaims created. Expected no more than %d, got %d", nodesAtStart+maxNodesDisrupting, len(nodeClaimList.Items))).Now()
		}

		// Don't include Nodes whose NodeClaims have been ignored
		nodeList.Items = lo.Filter(nodeList.Items, func(n corev1.Node, _ int) bool {
			return !removedProviderIDs.Has(n.Spec.ProviderID)
		})
		if len(nodeList.Items) > nodesAtStart+maxNodesDisrupting {
			StopTrying(fmt.Sprintf("Too many nodes created. Expected no more than %d, got %d", nodesAtStart+maxNodesDisrupting, len(nodeList.Items))).Now()
		}

		// Filter further by the number of tainted nodes to get the number of nodes that are disrupting
		nodes = lo.Filter(nodeList.Items, func(n corev1.Node, _ int) bool {
			_, ok := lo.Find(n.Spec.Taints, func(t corev1.Taint) bool {
				return t.MatchTaint(&karpv1.DisruptedNoScheduleTaint)
			})
			return ok
		})
		if len(nodes) > maxNodesDisrupting {
			StopTrying(fmt.Sprintf("Too many disruptions detected. Expected no more than %d, got %d", maxNodesDisrupting, len(nodeList.Items))).Now()
		}

		g.Expect(nodes).To(HaveLen(0))
	}).WithTimeout(timeout).WithPolling(5 * time.Second).Should(Succeed())
}

func (env *Environment) EventuallyExpectTaintedNodeCount(comparator string, count int) []*corev1.Node {
	GinkgoHelper()
	By(fmt.Sprintf("waiting for tainted nodes to be %s to %d", comparator, count))
	nodeList := &corev1.NodeList{}
	Eventually(func(g Gomega) {
		g.Expect(env.Client.List(env, nodeList, client.MatchingFields{"spec.taints[*].karpenter.sh/disrupted": "true"})).To(Succeed())
		g.Expect(len(nodeList.Items)).To(BeNumerically(comparator, count),
			fmt.Sprintf("expected %d tainted nodes, had %d (%v)", count, len(nodeList.Items), NodeNames(lo.ToSlicePtr(nodeList.Items))))
	}).Should(Succeed())
	return lo.ToSlicePtr(nodeList.Items)
}

func (env *Environment) EventuallyExpectNodesUntaintedWithTimeout(timeout time.Duration, nodes ...*corev1.Node) {
	GinkgoHelper()
	By(fmt.Sprintf("waiting for %d nodes to be untainted", len(nodes)))
	nodeList := &corev1.NodeList{}
	Eventually(func(g Gomega) {
		g.Expect(env.Client.List(env, nodeList, client.MatchingFields{"spec.taints[*].karpenter.sh/disrupted": "true"})).To(Succeed())
		taintedNodeNames := lo.Map(nodeList.Items, func(n corev1.Node, _ int) string { return n.Name })
		g.Expect(taintedNodeNames).ToNot(ContainElements(lo.Map(nodes, func(n *corev1.Node, _ int) interface{} { return n.Name })...))
	}).WithTimeout(timeout).Should(Succeed())
}

func (env *Environment) EventuallyExpectNodeClaimCount(comparator string, count int) []*karpv1.NodeClaim {
	GinkgoHelper()
	By(fmt.Sprintf("waiting for nodes to be %s to %d", comparator, count))
	nodeClaimList := &karpv1.NodeClaimList{}
	Eventually(func(g Gomega) {
		g.Expect(env.Client.List(env, nodeClaimList, client.HasLabels{test.DiscoveryLabel})).To(Succeed())
		g.Expect(len(nodeClaimList.Items)).To(BeNumerically(comparator, count),
			fmt.Sprintf("expected %d nodeclaims, had %d (%v)", count, len(nodeClaimList.Items), NodeClaimNames(lo.ToSlicePtr(nodeClaimList.Items))))
	}).Should(Succeed())
	return lo.ToSlicePtr(nodeClaimList.Items)
}

func (env *Environment) EventuallyExpectNodeCount(comparator string, count int) []*corev1.Node {
	GinkgoHelper()
	By(fmt.Sprintf("waiting for nodes to be %s to %d", comparator, count))
	nodeList := &corev1.NodeList{}
	Eventually(func(g Gomega) {
		g.Expect(env.Client.List(env, nodeList, client.HasLabels{test.DiscoveryLabel})).To(Succeed())
		g.Expect(len(nodeList.Items)).To(BeNumerically(comparator, count),
			fmt.Sprintf("expected %d nodes, had %d (%v)", count, len(nodeList.Items), NodeNames(lo.ToSlicePtr(nodeList.Items))))
	}).Should(Succeed())
	return lo.ToSlicePtr(nodeList.Items)
}

func (env *Environment) EventuallyExpectNodeCountWithSelector(comparator string, count int, selector labels.Selector) []*corev1.Node {
	GinkgoHelper()
	By(fmt.Sprintf("waiting for nodes with selector %v to be %s to %d", selector, comparator, count))
	nodeList := &corev1.NodeList{}
	Eventually(func(g Gomega) {
		g.Expect(env.Client.List(env, nodeList, client.HasLabels{test.DiscoveryLabel}, client.MatchingLabelsSelector{Selector: selector})).To(Succeed())
		g.Expect(len(nodeList.Items)).To(BeNumerically(comparator, count),
			fmt.Sprintf("expected %d nodes, had %d (%v)", count, len(nodeList.Items), NodeNames(lo.ToSlicePtr(nodeList.Items))))
	}).Should(Succeed())
	return lo.ToSlicePtr(nodeList.Items)
}

func (env *Environment) EventuallyExpectCreatedNodeCount(comparator string, count int) []*corev1.Node {
	GinkgoHelper()
	By(fmt.Sprintf("waiting for created nodes to be %s to %d", comparator, count))
	var createdNodes []*corev1.Node
	Eventually(func(g Gomega) {
		createdNodes = env.Monitor.CreatedNodes()
		g.Expect(len(createdNodes)).To(BeNumerically(comparator, count),
			fmt.Sprintf("expected %d created nodes, had %d (%v)", count, len(createdNodes), NodeNames(createdNodes)))
	}).Should(Succeed())
	return createdNodes
}

func (env *Environment) EventuallyExpectDeletedNodeCount(comparator string, count int) []*corev1.Node {
	GinkgoHelper()
	By(fmt.Sprintf("waiting for deleted nodes to be %s to %d", comparator, count))
	var deletedNodes []*corev1.Node
	Eventually(func(g Gomega) {
		deletedNodes = env.Monitor.DeletedNodes()
		g.Expect(len(deletedNodes)).To(BeNumerically(comparator, count),
			fmt.Sprintf("expected %d deleted nodes, had %d (%v)", count, len(deletedNodes), NodeNames(deletedNodes)))
	}).Should(Succeed())
	return deletedNodes
}

func (env *Environment) EventuallyExpectDeletedNodeCountWithSelector(comparator string, count int, selector labels.Selector) []*corev1.Node {
	GinkgoHelper()
	By(fmt.Sprintf("waiting for deleted nodes with selector %v to be %s to %d", selector, comparator, count))
	var deletedNodes []*corev1.Node
	Eventually(func(g Gomega) {
		deletedNodes = env.Monitor.DeletedNodes()
		deletedNodes = lo.Filter(deletedNodes, func(n *corev1.Node, _ int) bool {
			return selector.Matches(labels.Set(n.Labels))
		})
		g.Expect(len(deletedNodes)).To(BeNumerically(comparator, count),
			fmt.Sprintf("expected %d deleted nodes, had %d (%v)", count, len(deletedNodes), NodeNames(deletedNodes)))
	}).Should(Succeed())
	return deletedNodes
}

func (env *Environment) EventuallyExpectInitializedNodeCount(comparator string, count int) []*corev1.Node {
	GinkgoHelper()
	By(fmt.Sprintf("waiting for initialized nodes to be %s to %d", comparator, count))
	var nodes []*corev1.Node
	Eventually(func(g Gomega) {
		nodes = env.Monitor.CreatedNodes()
		nodes = lo.Filter(nodes, func(n *corev1.Node, _ int) bool {
			return n.Labels[karpv1.NodeInitializedLabelKey] == "true"
		})
		g.Expect(len(nodes)).To(BeNumerically(comparator, count))
	}).Should(Succeed())
	return nodes
}

func (env *Environment) EventuallyExpectCreatedNodeClaimCount(comparator string, count int) []*karpv1.NodeClaim {
	GinkgoHelper()
	By(fmt.Sprintf("waiting for created nodeclaims to be %s to %d", comparator, count))
	nodeClaimList := &karpv1.NodeClaimList{}
	Eventually(func(g Gomega) {
		g.Expect(env.Client.List(env.Context, nodeClaimList)).To(Succeed())
		g.Expect(len(nodeClaimList.Items)).To(BeNumerically(comparator, count))
	}).Should(Succeed())
	return lo.Map(nodeClaimList.Items, func(nc karpv1.NodeClaim, _ int) *karpv1.NodeClaim {
		return &nc
	})
}

func (env *Environment) EventuallyExpectNodeClaimsReady(nodeClaims ...*karpv1.NodeClaim) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		for _, nc := range nodeClaims {
			temp := &karpv1.NodeClaim{}
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nc), temp)).Should(Succeed())
			g.Expect(temp.StatusConditions().Root().IsTrue()).To(BeTrue())
		}
	}).Should(Succeed())
}

func (env *Environment) EventuallyExpectDrifted(nodeClaims ...*karpv1.NodeClaim) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		for _, nc := range nodeClaims {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(nc), nc)).To(Succeed())
			g.Expect(nc.StatusConditions().Get(karpv1.ConditionTypeDrifted).IsTrue()).To(BeTrue())
		}
	}).Should(Succeed())
}

func (env *Environment) ConsistentlyExpectNodeClaimsNotDrifted(duration time.Duration, nodeClaims ...*karpv1.NodeClaim) {
	GinkgoHelper()
	nodeClaimNames := lo.Map(nodeClaims, func(nc *karpv1.NodeClaim, _ int) string { return nc.Name })
	By(fmt.Sprintf("consistently expect nodeclaims %s not to be drifted for %s", nodeClaimNames, duration))
	Consistently(func(g Gomega) {
		for _, nc := range nodeClaims {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(nc), nc)).To(Succeed())
			g.Expect(nc.StatusConditions().Get(karpv1.ConditionTypeDrifted)).To(BeNil())
		}
	}, duration).Should(Succeed())
}

func (env *Environment) EventuallyExpectConsolidatable(nodeClaims ...*karpv1.NodeClaim) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		for _, nc := range nodeClaims {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(nc), nc)).To(Succeed())
			g.Expect(nc.StatusConditions().Get(karpv1.ConditionTypeConsolidatable).IsTrue()).To(BeTrue())
		}
	}).Should(Succeed())
}

func (env *Environment) GetNode(nodeName string) corev1.Node {
	GinkgoHelper()
	var node corev1.Node
	Expect(env.Client.Get(env.Context, types.NamespacedName{Name: nodeName}, &node)).To(Succeed())
	return node
}

func (env *Environment) ExpectNoCrashes() {
	GinkgoHelper()
	for k, v := range env.Monitor.RestartCount("kube-system") {
		if strings.Contains(k, "karpenter") && v > 0 {
			Fail("expected karpenter containers to not crash")
		}
	}
}

var (
	lastLogged = metav1.Now()
)

func (env *Environment) printControllerLogs(options *corev1.PodLogOptions) {
	fmt.Println("------- START CONTROLLER LOGS -------")
	defer fmt.Println("------- END CONTROLLER LOGS -------")

	if options.SinceTime == nil {
		options.SinceTime = lastLogged.DeepCopy()
		lastLogged = metav1.Now()
	}
	pods := env.ExpectKarpenterPods()
	for _, pod := range pods {
		temp := options.DeepCopy() // local version of the log options

		fmt.Printf("------- pod/%s -------\n", pod.Name)
		if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].RestartCount > 0 {
			fmt.Printf("[PREVIOUS CONTAINER LOGS]\n")
			temp.Previous = true
		}
		stream, err := env.KubeClient.CoreV1().Pods("kube-system").GetLogs(pod.Name, temp).Stream(env.Context)
		if err != nil {
			log.FromContext(env.Context).Error(err, "failed fetching controller logs")
			return
		}
		raw := &bytes.Buffer{}
		_, err = io.Copy(raw, stream)
		Expect(err).ToNot(HaveOccurred())
		log.FromContext(env.Context).Info(raw.String())
	}
}

func (env *Environment) EventuallyExpectMinUtilization(resource corev1.ResourceName, comparator string, value float64) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		g.Expect(env.Monitor.MinUtilization(resource)).To(BeNumerically(comparator, value))
	}).Should(Succeed())
}

func (env *Environment) EventuallyExpectAvgUtilization(resource corev1.ResourceName, comparator string, value float64) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		g.Expect(env.Monitor.AvgUtilization(resource)).To(BeNumerically(comparator, value))
	}, 12*time.Minute).Should(Succeed())
}

func (env *Environment) ExpectDaemonSetEnvironmentVariableUpdated(obj client.ObjectKey, name, value string, containers ...string) {
	GinkgoHelper()
	ds := &appsv1.DaemonSet{}
	Expect(env.Client.Get(env.Context, obj, ds)).To(Succeed())
	if len(containers) == 0 {
		Expect(len(ds.Spec.Template.Spec.Containers)).To(BeNumerically("==", 1))
		containers = append(containers, ds.Spec.Template.Spec.Containers[0].Name)
	}
	patch := client.StrategicMergeFrom(ds.DeepCopy())
	containerNames := sets.New(containers...)
	for ci := range ds.Spec.Template.Spec.Containers {
		c := &ds.Spec.Template.Spec.Containers[ci]
		if !containerNames.Has(c.Name) {
			continue
		}
		// If the env var already exists, update its value. Otherwise, create a new var.
		if _, i, ok := lo.FindIndexOf(c.Env, func(e corev1.EnvVar) bool {
			return e.Name == name
		}); ok {
			c.Env[i].Value = value
		} else {
			c.Env = append(c.Env, corev1.EnvVar{Name: name, Value: value})
		}
	}
	Expect(env.Client.Patch(env.Context, ds, patch)).To(Succeed())
}

// ForcePodsToSpread ensures that currently scheduled pods get spread evenly across all passed nodes by deleting pods off of existing
// nodes and waiting them to reschedule. This is useful for scenarios where you want to force the nodes be underutilized
// but you want to keep a consistent count of nodes rather than leaving around empty ones.
func (env *Environment) ForcePodsToSpread(nodes ...*corev1.Node) {
	GinkgoHelper()

	// Get the total count of pods across
	podCount := 0
	for _, n := range nodes {
		podCount += len(env.ExpectActivePodsForNode(n.Name))
	}
	maxPodsPerNode := int(math.Ceil(float64(podCount) / float64(len(nodes))))

	By(fmt.Sprintf("forcing %d pods to spread across %d nodes", podCount, len(nodes)))
	start := time.Now()
	for {
		var nodePods []*corev1.Pod
		node, found := lo.Find(nodes, func(n *corev1.Node) bool {
			nodePods = env.ExpectActivePodsForNode(n.Name)
			return len(nodePods) > maxPodsPerNode
		})
		if !found {
			break
		}
		// Set the nodes to unschedulable so that the pods won't reschedule.
		Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(node), node)).To(Succeed())
		stored := node.DeepCopy()
		node.Spec.Unschedulable = true
		Expect(env.Client.Patch(env.Context, node, client.StrategicMergeFrom(stored))).To(Succeed())
		for _, pod := range nodePods[maxPodsPerNode:] {
			env.ExpectDeleted(pod)
		}
		Eventually(func(g Gomega) {
			g.Expect(len(env.ExpectActivePodsForNode(node.Name))).To(Or(Equal(maxPodsPerNode), Equal(maxPodsPerNode-1)))
		}).WithTimeout(5 * time.Second).Should(Succeed())

		// TODO: Consider moving this time check to an Eventually poll. This gets a little tricker with helper functions
		// since you need to make sure that your Expectation helper functions are scoped to to your "g Gomega" scope
		// so that you don't fail the first time you get a failure on your expectation
		if time.Since(start) > time.Minute*15 {
			Fail("forcing pods to spread failed due to a timeout")
		}
	}
	for _, n := range nodes {
		stored := n.DeepCopy()
		n.Spec.Unschedulable = false
		Expect(env.Client.Patch(env.Context, n, client.StrategicMergeFrom(stored))).To(Succeed())
	}
}

func (env *Environment) ExpectActivePodsForNode(nodeName string) []*corev1.Pod {
	GinkgoHelper()
	podList := &corev1.PodList{}
	Expect(env.Client.List(env, podList, client.MatchingFields{"spec.nodeName": nodeName}, client.HasLabels{test.DiscoveryLabel})).To(Succeed())

	return lo.Filter(lo.ToSlicePtr(podList.Items), func(p *corev1.Pod, _ int) bool {
		return p.DeletionTimestamp.IsZero()
	})
}

func (env *Environment) ExpectCABundle() string {
	// Discover CA Bundle from the REST client. We could alternatively
	// have used the simpler client-go InClusterConfig() method.
	// However, that only works when Karpenter is running as a Pod
	// within the same cluster it's managing.
	GinkgoHelper()
	transportConfig, err := env.Config.TransportConfig()
	Expect(err).ToNot(HaveOccurred())
	_, err = transport.TLSConfigFor(transportConfig) // fills in CAData!
	Expect(err).ToNot(HaveOccurred())
	log.FromContext(env.Context).WithValues("length", len(transportConfig.TLS.CAData)).V(1).Info("discovered caBundle")
	return base64.StdEncoding.EncodeToString(transportConfig.TLS.CAData)
}

func (env *Environment) GetDaemonSetCount(np *karpv1.NodePool) int {
	GinkgoHelper()

	// Performs the same logic as the scheduler to get the number of daemonset
	// pods that we estimate we will need to schedule as overhead to each node
	daemonSetList := &appsv1.DaemonSetList{}
	Expect(env.Client.List(env.Context, daemonSetList)).To(Succeed())

	return lo.CountBy(daemonSetList.Items, func(d appsv1.DaemonSet) bool {
		p := &corev1.Pod{Spec: d.Spec.Template.Spec}
		nodeClaimTemplate := pscheduling.NewNodeClaimTemplate(np)
		if err := scheduling.Taints(nodeClaimTemplate.Spec.Taints).ToleratesPod(p); err != nil {
			return false
		}
		if err := nodeClaimTemplate.Requirements.Compatible(scheduling.NewPodRequirements(p), scheduling.AllowUndefinedWellKnownLabels); err != nil {
			return false
		}
		return true
	})
}

func (env *Environment) GetDaemonSetOverhead(np *karpv1.NodePool) corev1.ResourceList {
	GinkgoHelper()

	// Performs the same logic as the scheduler to get the number of daemonset
	// pods that we estimate we will need to schedule as overhead to each node
	daemonSetList := &appsv1.DaemonSetList{}
	Expect(env.Client.List(env.Context, daemonSetList)).To(Succeed())

	return coreresources.RequestsForPods(lo.FilterMap(daemonSetList.Items, func(ds appsv1.DaemonSet, _ int) (*corev1.Pod, bool) {
		p := &corev1.Pod{Spec: ds.Spec.Template.Spec}
		nodeClaimTemplate := pscheduling.NewNodeClaimTemplate(np)
		if err := scheduling.Taints(nodeClaimTemplate.Spec.Taints).ToleratesPod(p); err != nil {
			return nil, false
		}
		if err := nodeClaimTemplate.Requirements.Compatible(scheduling.NewPodRequirements(p), scheduling.AllowUndefinedWellKnownLabels); err != nil {
			return nil, false
		}
		return p, true
	})...)
}
