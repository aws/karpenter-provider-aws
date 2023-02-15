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

package daemonset

import (
	"testing"

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
	environmentaws "github.com/aws/karpenter/test/pkg/environment/aws"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var env *environmentaws.Environment
var namespace *v1.Namespace
var limitrange *v1.LimitRange
var priorityclass *schedulingv1.PriorityClass

const DiscoveryLabel = v1alpha5.TestingGroup + "/test-id"

func TestDaemonsets(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		env = environmentaws.NewEnvironment(t)
	})
	AfterSuite(func() {
		env.Stop()
	})
	RunSpecs(t, "DaemonSet")
}

var _ = BeforeEach(func() { env.BeforeEach() })
var _ = AfterEach(func() { env.Cleanup() })
var _ = AfterEach(func() { env.ForceCleanup() })
var _ = AfterEach(func() { env.AfterEach() })

var _ = Describe("DaemonSet", func() {
	It("should account for LimitRange Containers Default For Resources", func() {
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
		})
		preemptNever := v1.PreemptNever
		priorityclass = &schedulingv1.PriorityClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: "high-priority-daemonsets",
			},
			PreemptionPolicy: &preemptNever,
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
				Limits: []v1.LimitRangeItem{{
					Type: v1.LimitTypeContainer,
					Default: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("2"),
						v1.ResourceMemory: resource.MustParse("1Gi"),
					}}},
			},
		}
		daemonset := test.DaemonSet(test.DaemonSetOptions{
			PodOptions: test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{Limits: v1.ResourceList{v1.ResourceMemory: resource.MustParse("1Gi")}},
				PriorityClassName:    "high-priority-daemonsets",
			},
		})
		numPods := 1
		dep := test.Deployment(test.DeploymentOptions{
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

		podSelector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		daemonsetSector := labels.SelectorFromSet(daemonset.Spec.Selector.MatchLabels)
		env.ExpectCreatedNodeCount("==", 0)
		env.ExpectCreated(provisioner, provider, limitrange, priorityclass, daemonset, dep)
		env.EventuallyExpectHealthyPodCount(podSelector, 1)
		env.EventuallyExpectHealthyPodCount(daemonsetSector, 2)
		env.ExpectCreatedNodeCount("==", 2)
	})
	It("should account for LimitRange Containers Default Requests For Resources", func() {
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
		})
		preemptNever := v1.PreemptNever
		priorityclass = &schedulingv1.PriorityClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: "high-priority-daemonsets",
			},
			PreemptionPolicy: &preemptNever,
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
				Limits: []v1.LimitRangeItem{{
					Type: v1.LimitTypeContainer,
					DefaultRequest: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("2"),
						v1.ResourceMemory: resource.MustParse("1Gi"),
					}}},
			},
		}
		daemonset := test.DaemonSet(test.DaemonSetOptions{
			PodOptions: test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{Limits: v1.ResourceList{v1.ResourceMemory: resource.MustParse("1Gi")}},
				PriorityClassName:    "high-priority-daemonsets",
			},
		})
		numPods := 1
		dep := test.Deployment(test.DeploymentOptions{
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

		podSelector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		daemonsetSector := labels.SelectorFromSet(daemonset.Spec.Selector.MatchLabels)
		env.ExpectCreatedNodeCount("==", 0)
		env.ExpectCreated(provisioner, provider, limitrange, priorityclass, daemonset, dep)
		env.EventuallyExpectHealthyPodCount(podSelector, 1)
		env.EventuallyExpectHealthyPodCount(daemonsetSector, 2)
		env.ExpectCreatedNodeCount("==", 2)
	})
})
