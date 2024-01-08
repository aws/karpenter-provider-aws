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

package integration_test

import (
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	"sigs.k8s.io/karpenter/pkg/test"
)

var _ = Describe("DaemonSet", func() {
	var limitrange *v1.LimitRange
	var priorityclass *schedulingv1.PriorityClass
	var daemonset *appsv1.DaemonSet
	var dep *appsv1.Deployment

	BeforeEach(func() {
		nodePool.Spec.Disruption.ConsolidationPolicy = corev1beta1.ConsolidationPolicyWhenUnderutilized
		nodePool.Spec.Disruption.ConsolidateAfter = nil
		priorityclass = &schedulingv1.PriorityClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: "high-priority-daemonsets",
			},
			Value:         int32(10000000),
			GlobalDefault: false,
			Description:   "This priority class should be used for daemonsets.",
		}
		limitrange = &v1.LimitRange{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "limitrange",
				Namespace: "default",
			},
		}
		daemonset = test.DaemonSet(test.DaemonSetOptions{
			PodOptions: test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{Limits: v1.ResourceList{v1.ResourceMemory: resource.MustParse("1Gi")}},
				PriorityClassName:    "high-priority-daemonsets",
			},
		})
		numPods := 1
		dep = test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "large-app"},
				},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceMemory: resource.MustParse("4")},
				},
			},
		})
	})
	It("should account for LimitRange Default on daemonSet pods for resources", func() {
		limitrange.Spec.Limits = []v1.LimitRangeItem{
			{
				Type: v1.LimitTypeContainer,
				Default: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("2"),
					v1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
		}

		podSelector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		daemonSetSelector := labels.SelectorFromSet(daemonset.Spec.Selector.MatchLabels)
		env.ExpectCreated(nodeClass, nodePool, limitrange, priorityclass, daemonset, dep)

		// Eventually expect a single node to exist and both the deployment pod and the daemonset pod to schedule to it
		Eventually(func(g Gomega) {
			nodeList := &v1.NodeList{}
			g.Expect(env.Client.List(env, nodeList, client.HasLabels{"testing/cluster"})).To(Succeed())
			g.Expect(nodeList.Items).To(HaveLen(1))

			deploymentPods := env.Monitor.RunningPods(podSelector)
			g.Expect(deploymentPods).To(HaveLen(1))

			daemonSetPods := env.Monitor.RunningPods(daemonSetSelector)
			g.Expect(daemonSetPods).To(HaveLen(1))

			g.Expect(deploymentPods[0].Spec.NodeName).To(Equal(nodeList.Items[0].Name))
			g.Expect(daemonSetPods[0].Spec.NodeName).To(Equal(nodeList.Items[0].Name))
		}).Should(Succeed())
	})
	It("should account for LimitRange DefaultRequest on daemonSet pods for resources", func() {
		limitrange.Spec.Limits = []v1.LimitRangeItem{
			{
				Type: v1.LimitTypeContainer,
				DefaultRequest: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("2"),
					v1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
		}

		podSelector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		daemonSetSelector := labels.SelectorFromSet(daemonset.Spec.Selector.MatchLabels)
		env.ExpectCreated(nodeClass, nodePool, limitrange, priorityclass, daemonset, dep)

		// Eventually expect a single node to exist and both the deployment pod and the daemonset pod to schedule to it
		Eventually(func(g Gomega) {
			nodeList := &v1.NodeList{}
			g.Expect(env.Client.List(env, nodeList, client.HasLabels{"testing/cluster"})).To(Succeed())
			g.Expect(nodeList.Items).To(HaveLen(1))

			deploymentPods := env.Monitor.RunningPods(podSelector)
			g.Expect(deploymentPods).To(HaveLen(1))

			daemonSetPods := env.Monitor.RunningPods(daemonSetSelector)
			g.Expect(daemonSetPods).To(HaveLen(1))

			g.Expect(deploymentPods[0].Spec.NodeName).To(Equal(nodeList.Items[0].Name))
			g.Expect(daemonSetPods[0].Spec.NodeName).To(Equal(nodeList.Items[0].Name))
		}).Should(Succeed())
	})
})
