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
	"fmt"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	karpv1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	coretest "sigs.k8s.io/karpenter/pkg/test"
)

var _ = FDescribe("EC2NodeClass Migration Controller", func() {
	BeforeEach(func() {
		nodeClass = env.DefaultEC2NodeClass()
		nodePool = env.DefaultNodePool(nodeClass)
	})
	It("should have migration key present for v1 resources and absent for v1beta1 resources", func() {
		pod := coretest.Pod()
		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		nodeClaim := env.EventuallyExpectNodeClaimCount("==", 1)[0]
		Eventually(func(g Gomega) {
			ec2nc := &v1.EC2NodeClass{}
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClass), ec2nc)).To(Succeed())
			g.Expect(ec2nc.GetAnnotations()).To(HaveKey(karpv1.StoredVersionMigratedKey))
		})
		Eventually(func(g Gomega) {
			np := &karpv1.NodePool{}
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodePool), np)).To(Succeed())
			g.Expect(np.GetAnnotations()).To(HaveKey(karpv1.StoredVersionMigratedKey))
		})
		Eventually(func(g Gomega) {
			nc := &karpv1.NodeClaim{}
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClaim), nc)).To(Succeed())
			g.Expect(nc.GetAnnotations()).To(HaveKey(karpv1.StoredVersionMigratedKey))
		})
		Eventually(func(g Gomega) {
			ec2nc := &v1beta1.EC2NodeClass{}
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClass), ec2nc)).To(Succeed())
			g.Expect(ec2nc.GetAnnotations()).To(Not(HaveKey(karpv1.StoredVersionMigratedKey)))
		})
		Eventually(func(g Gomega) {
			np := &karpv1beta1.NodePool{}
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodePool), np)).To(Succeed())
			g.Expect(np.GetAnnotations()).To(Not(HaveKey(karpv1.StoredVersionMigratedKey)))
		})
		Eventually(func(g Gomega) {
			nc := &karpv1beta1.NodeClaim{}
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClaim), nc)).To(Succeed())
			g.Expect(nc.GetAnnotations()).To(Not(HaveKey(karpv1.StoredVersionMigratedKey)))
		})
	})
	It("should update CRD status stored versions", func() {
		for _, item := range apis.CRDs {
			crd := &apiextensionsv1.CustomResourceDefinition{}
			Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(item), crd)).To(Succeed())
			stored := crd.DeepCopy()
			crd.Status.StoredVersions = []string{"v1beta1"}
			Eventually(func(g Gomega) {
				g.Expect(env.Client.Status().Patch(env.Context, crd, client.StrategicMergeFrom(stored, client.MergeFromWithOptimisticLock{}))).To(Succeed())
			}).WithTimeout(time.Second * 10).Should(Succeed())
		}
		for _, item := range apis.CRDs {
			crd := &apiextensionsv1.CustomResourceDefinition{}
			Eventually(func(g Gomega) {
				g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(item), crd)).To(Succeed())
				fmt.Println(crd.Status.StoredVersions)
				g.Expect(crd.Status.StoredVersions).To(HaveExactElements("v1"))
			}).WithTimeout(time.Second * 10).Should(Succeed())
		}
	})
})
