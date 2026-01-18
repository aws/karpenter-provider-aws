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

package daemonset_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"sigs.k8s.io/karpenter/pkg/test"
	"sigs.k8s.io/karpenter/pkg/utils/daemonset"
)

func TestReconciles(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DaemonSetUtils")
}

var _ = Describe("DaemonSetUtils", func() {
	It("should merge resource limits into requests if no requests exists for the given container", func() {
		inputRequirements := corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("1000Mi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("750m"),
			},
		}
		expectedRequirements := corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("1000Mi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("750m"),
				corev1.ResourceMemory: resource.MustParse("1000Mi"),
			},
		}

		daemonSet := test.DaemonSet(test.DaemonSetOptions{
			PodOptions: test.PodOptions{
				ResourceRequirements: inputRequirements,
				InitContainers: []corev1.Container{{
					Resources: inputRequirements,
				}},
			},
		})
		p := daemonset.PodForDaemonSet(daemonSet)
		Expect(p.Spec.Containers).To(HaveLen(1))
		Expect(p.Spec.Containers[0].Resources).To(Equal(expectedRequirements))
		Expect(p.Spec.InitContainers).To(HaveLen(1))
		Expect(p.Spec.InitContainers[0].Resources).To(Equal(expectedRequirements))
	})
})
