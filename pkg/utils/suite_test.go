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

package utils_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
)

func TestUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils Suite")
}

var _ = Describe("GetNodeClassHash", func() {
	It("should return formatted hash with UID and Generation", func() {
		nodeClass := &v1.EC2NodeClass{
			ObjectMeta: metav1.ObjectMeta{
				UID:        "test-uid-123",
				Generation: 5,
			},
		}
		hash := utils.GetNodeClassHash(nodeClass)
		Expect(hash).To(Equal("test-uid-123-5"))
	})
})

var _ = Describe("GetTags", func() {
	var nodeClass *v1.EC2NodeClass
	var nodeClaim *karpv1.NodeClaim
	BeforeEach(func() {
		nodeClass = &v1.EC2NodeClass{ObjectMeta: metav1.ObjectMeta{Name: "test-nodeclass"}}
		nodeClaim = &karpv1.NodeClaim{ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{karpv1.NodePoolLabelKey: "test-nodepool"},
		}}
	})
	It("should tag resources with the default cluster-name tag key", func() {
		tags, err := utils.GetTags(nodeClass, nodeClaim, "test-cluster", v1.EKSClusterNameTagKey)
		Expect(err).ToNot(HaveOccurred())
		Expect(tags).To(HaveKeyWithValue(v1.EKSClusterNameTagKey, "test-cluster"))
	})
	It("should tag resources with a custom cluster-name tag key", func() {
		tags, err := utils.GetTags(nodeClass, nodeClaim, "test-cluster", "rosa:rosa-cluster-name")
		Expect(err).ToNot(HaveOccurred())
		Expect(tags).To(HaveKeyWithValue("rosa:rosa-cluster-name", "test-cluster"))
		Expect(tags).ToNot(HaveKey(v1.EKSClusterNameTagKey))
	})
	It("should reject user-supplied tags matching the configured cluster-name tag key", func() {
		nodeClass.Spec.Tags = map[string]string{"rosa:rosa-cluster-name": "user-value"}
		_, err := utils.GetTags(nodeClass, nodeClaim, "test-cluster", "rosa:rosa-cluster-name")
		Expect(err).To(HaveOccurred())
	})
	It("should still reject the default eks:eks-cluster-name tag even when a custom key is configured", func() {
		nodeClass.Spec.Tags = map[string]string{v1.EKSClusterNameTagKey: "user-value"}
		_, err := utils.GetTags(nodeClass, nodeClaim, "test-cluster", "rosa:rosa-cluster-name")
		Expect(err).To(HaveOccurred())
	})
})
