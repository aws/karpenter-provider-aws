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
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CRD Hash", func() {
	It("should have NodePool hash", func() {
		env.ExpectCreated(nodeClass, nodePool)

		Eventually(func(g Gomega) {
			np := &corev1beta1.NodePool{}
			err := env.Client.Get(env, client.ObjectKeyFromObject(nodePool), np)
			g.Expect(err).ToNot(HaveOccurred())

			hash, found := np.Annotations[corev1beta1.NodePoolHashAnnotationKey]
			g.Expect(found).To(BeTrue())
			g.Expect(hash).To(Equal(np.Hash()))
		})
	})
	It("should have EC2NodeClass hash", func() {
		env.ExpectCreated(nodeClass)

		Eventually(func(g Gomega) {
			nc := &v1beta1.EC2NodeClass{}
			err := env.Client.Get(env, client.ObjectKeyFromObject(nodeClass), nc)
			g.Expect(err).ToNot(HaveOccurred())

			hash, found := nc.Annotations[v1beta1.AnnotationEC2NodeClassHash]
			g.Expect(found).To(BeTrue())
			g.Expect(hash).To(Equal(nc.Hash()))
		})
	})
})
