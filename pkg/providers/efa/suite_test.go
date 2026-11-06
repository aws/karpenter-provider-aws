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

package efa_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/efa"
	"github.com/aws/karpenter-provider-aws/pkg/providers/efadra"
)

func TestEFA(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EFA")
}

var _ = Describe("DefaultProvider", func() {
	var nodeClaim *karpv1.NodeClaim
	BeforeEach(func() {
		nodeClaim = &karpv1.NodeClaim{}
	})

	It("should enable EFA when the NodeClaim requests the EFA extended resource", func() {
		nodeClaim.Spec.Resources.Requests = corev1.ResourceList{v1.ResourceEFA: resource.MustParse("1")}
		Expect(efa.DefaultProvider{}.Enabled(nodeClaim, &v1.EC2NodeClass{})).To(BeTrue())
	})
	It("should enable EFA when dra.net devices were allocated to the NodeClaim", func() {
		nodeClaim.ObjectMeta = metav1.ObjectMeta{Annotations: map[string]string{karpv1.DRADriversAnnotationKey: "gpu.nvidia.com," + efadra.DriverName}}
		Expect(efa.DefaultProvider{}.Enabled(nodeClaim, &v1.EC2NodeClass{})).To(BeTrue())
	})
	It("should not enable EFA otherwise", func() {
		nodeClaim.Spec.Resources.Requests = corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")}
		nodeClaim.ObjectMeta = metav1.ObjectMeta{Annotations: map[string]string{karpv1.DRADriversAnnotationKey: "gpu.nvidia.com"}}
		Expect(efa.DefaultProvider{}.Enabled(nodeClaim, &v1.EC2NodeClass{})).To(BeFalse())
	})
})
