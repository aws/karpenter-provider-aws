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
	"github.com/samber/lo"

	v1 "github.com/aws/karpenter/pkg/apis/v1"
	"github.com/aws/karpenter/pkg/apis/v1beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	karpv1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	coretest "sigs.k8s.io/karpenter/pkg/test"
)

var _ = Describe("Migration", func() {
	BeforeEach(func() {
		nodeClass = env.DefaultEC2NodeClass()
		nodePool = env.DefaultNodePool(nodeClass)
	})
	It("should not have migration key present for v1beta1 resources", func() {
		pod := coretest.Pod()
		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		Eventually(func(g Gomega) {
			stored := nodeClass.DeepCopy()
			nodeClass.Annotations = lo.Assign(nodeClass.Annotations, map[string]string{
				karpv1.StoredVersionMigratedKey: "true",
			})
			g.Expect(env.Client.Patch(env.Context, nodeClass, client.StrategicMergeFrom(stored, client.MergeFromWithOptimisticLock{}))).To(Succeed())
			ec2nc := &v1beta1.EC2NodeClass{}
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClass), ec2nc)).To(Succeed())
			v1ec2nc := &v1.EC2NodeClass{}
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClass), v1ec2nc)).To(Succeed())
			g.Expect(v1ec2nc.GetAnnotations()).To(HaveKey(karpv1.StoredVersionMigratedKey))
			g.Expect(ec2nc.GetAnnotations()).To(Not(HaveKey(karpv1.StoredVersionMigratedKey)))
		})
		Eventually(func(g Gomega) {
			stored := nodePool.DeepCopy()
			nodePool.Annotations = lo.Assign(nodePool.Annotations, map[string]string{
				karpv1.StoredVersionMigratedKey: "true",
			})
			g.Expect(env.Client.Patch(env.Context, nodePool, client.StrategicMergeFrom(stored, client.MergeFromWithOptimisticLock{}))).To(Succeed())
			np := &karpv1beta1.NodePool{}
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodePool), np)).To(Succeed())
			v1np := &karpv1.NodePool{}
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodePool), v1np)).To(Succeed())
			g.Expect(v1np.GetAnnotations()).To(HaveKey(karpv1.StoredVersionMigratedKey))
			g.Expect(np.GetAnnotations()).To(Not(HaveKey(karpv1.StoredVersionMigratedKey)))
		})
		Eventually(func(g Gomega) {
			stored := nodeClaim.DeepCopy()
			nodeClaim.Annotations = lo.Assign(nodeClaim.Annotations, map[string]string{
				karpv1.StoredVersionMigratedKey: "true",
			})
			g.Expect(env.Client.Patch(env.Context, nodeClaim, client.StrategicMergeFrom(stored, client.MergeFromWithOptimisticLock{}))).To(Succeed())
			nc := &karpv1beta1.NodeClaim{}
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClaim), nc)).To(Succeed())
			v1nc := &karpv1.NodeClaim{}
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClaim), v1nc)).To(Succeed())
			g.Expect(v1nc.GetAnnotations()).To(HaveKey(karpv1.StoredVersionMigratedKey))
			g.Expect(nc.GetAnnotations()).To(Not(HaveKey(karpv1.StoredVersionMigratedKey)))
		})
	})
})
