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

package performance

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/karpenter/pkg/test"
)

var _ = Describe("Performance", func() {
	Context("Provisioning", func() {
		It("should provision a node for a pending pod", func() {
			var replicas = 1
			deployment := test.Deployment(test.DeploymentOptions{
				Replicas: int32(replicas),
				PodOptions: test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: testLabels,
					},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
					},
				}})
			env.ExpectCreated(deployment)
			env.ExpectCreated(nodePool, nodeClass)
			env.EventuallyExpectHealthyPodCountWithTimeout(10*time.Minute, labelSelector, replicas)
		})
	})
})
