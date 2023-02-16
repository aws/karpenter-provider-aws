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

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	awstest "github.com/aws/karpenter/pkg/test"
	"github.com/samber/lo"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DaemonSet", func() {
	var provider *v1alpha1.AWSNodeTemplate
	var provisioner *v1alpha5.Provisioner
	var limitrange *v1.LimitRange
	var priorityclass *schedulingv1.PriorityClass
	var daemonset *appsv1.DaemonSet
	var dep *appsv1.Deployment

	BeforeEach(func() {
		provider = awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})
		provisioner = test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
			Consolidation: &v1alpha5.Consolidation{
				Enabled: lo.ToPtr(true),
			},
		})
		priorityclass = &schedulingv1.PriorityClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: "high-priority-daemonsets",
			},
			PreemptionPolicy: lo.ToPtr(v1.PreemptNever),
			Value:            int32(10000000),
			GlobalDefault:    false,
			Description:      "This priority class should be used for daemonsets.",
		}
		limitrange = &v1.LimitRange{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "limitrange",
				Namespace: "default",
			},
			Spec: v1.LimitRangeSpec{
				Limits: []v1.LimitRangeItem{},
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
	It("should account for LimitRange Defaults on daemonSet pods For Resources", func() {
		defaultLimit := v1.LimitRangeItem{
			Type: v1.LimitTypeContainer,
			Default: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("2"),
				v1.ResourceMemory: resource.MustParse("1Gi"),
			},
		}
		limitrange.Spec.Limits = append(limitrange.Spec.Limits, defaultLimit)

		podSelector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		daemonSetSelector := labels.SelectorFromSet(daemonset.Spec.Selector.MatchLabels)
		env.ExpectCreated(provisioner, provider, limitrange, priorityclass, daemonset, dep)
		env.EventuallyExpectHealthyPodCount(podSelector, 1)
		env.EventuallyExpectHealthyPodCount(daemonSetSelector, 1)
		EventuallyExpectOneNodeWithAllPods(podSelector, daemonSetSelector)
	})
	It("should account for LimitRange Default Requests on daemonSet pods For Resources", func() {
		defaultRequestLimit := v1.LimitRangeItem{
			Type: v1.LimitTypeContainer,
			DefaultRequest: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("2"),
				v1.ResourceMemory: resource.MustParse("1Gi"),
			},
		}
		limitrange.Spec.Limits = append(limitrange.Spec.Limits, defaultRequestLimit)

		podSelector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		daemonSetSelector := labels.SelectorFromSet(daemonset.Spec.Selector.MatchLabels)
		env.ExpectCreated(provisioner, provider, limitrange, priorityclass, daemonset, dep)
		env.EventuallyExpectHealthyPodCount(podSelector, 1)
		env.EventuallyExpectHealthyPodCount(daemonSetSelector, 1)
		EventuallyExpectOneNodeWithAllPods(podSelector, daemonSetSelector)
	})
})

func EventuallyExpectOneNodeWithAllPods(podSelector labels.Selector, daemonSetSelector labels.Selector) {
	env.EventuallyExpectCreatedNodeCount("==", 1)
	createdNode := &v1.Node{}

	for _, node := range env.Monitor.Nodes() {
		if lo.Contains(lo.Keys(node.Labels), "testing.karpenter.sh/test-id") {
			createdNode = node
			break
		}
	}

	pod := env.Monitor.RunningPods(podSelector)
	daemonSetPod := env.Monitor.RunningPods(daemonSetSelector)

	EventuallyWithOffset(1, func(g Gomega) {
		Expect(pod[0].Spec.NodeName).To(Equal(createdNode.Name))
	}).Should(Succeed())

	EventuallyWithOffset(1, func(g Gomega) {
		Expect(daemonSetPod[0].Spec.NodeName).To(Equal(createdNode.Name))
	}).Should(Succeed())
}
