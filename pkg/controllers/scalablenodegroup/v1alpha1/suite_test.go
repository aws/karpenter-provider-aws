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

package v1alpha1

import (
	"fmt"
	"testing"

	v1alpha1 "github.com/awslabs/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/cloudprovider/fake"
	"knative.dev/pkg/ptr"

	"github.com/awslabs/karpenter/pkg/test/environment"
	. "github.com/awslabs/karpenter/pkg/test/expectations"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t,
		"ScalableNodeGroup",
		[]Reporter{printer.NewlineReporter{}})
}

var fakeCloudProvider = fake.NewFactory(cloudprovider.Options{})
var fakeController = &Controller{CloudProvider: fakeCloudProvider}
var env environment.Environment = environment.NewLocal(func(e *environment.Local) {
	e.Manager.Register(fakeController)
})

var _ = BeforeSuite(func() {
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Examples", func() {
	var ns *environment.Namespace
	var defaultReplicaCount = ptr.Int32(3)

	var sng *v1alpha1.ScalableNodeGroup

	BeforeEach(func() {
		var err error
		ns, err = env.NewNamespace()
		Expect(err).NotTo(HaveOccurred())
		sng = &v1alpha1.ScalableNodeGroup{}
		fakeCloudProvider.NodeGroupStable = true
		fakeCloudProvider.NodeGroupMessage = ""
		fakeCloudProvider.WantErr = nil
		sng.Spec.Replicas = defaultReplicaCount
		fakeCloudProvider.NodeReplicas[sng.Spec.ID] = ptr.Int32(0)
	})

	Context("ScalableNodeGroup", func() {
		It("should be created", func() {
			Expect(ns.ParseResources("docs/examples/reserved-capacity-utilization.yaml", sng)).To(Succeed())
			fakeCloudProvider.NodeReplicas[sng.Spec.ID] = ptr.Int32(*sng.Spec.Replicas)
			ExpectCreated(ns.Client, sng)
			ExpectEventuallyHappy(ns.Client, sng)
			ExpectDeleted(ns.Client, sng)
		})
	})

	Context("ScalableNodeGroup Reconcile tests", func() {
		It("Test reconciler to scale up nodes", func() {
			Expect(fakeController.Reconcile(sng)).To(Succeed())
			Expect(fakeCloudProvider.NodeReplicas[sng.Spec.ID]).To(Equal(defaultReplicaCount))
		})

		It("Test reconciler to scale down nodes", func() {
			fakeCloudProvider.NodeReplicas[sng.Spec.ID] = ptr.Int32(10) // set existing replicas higher than desired
			Expect(fakeController.Reconcile(sng)).To(Succeed())
			Expect(fakeCloudProvider.NodeReplicas[sng.Spec.ID]).To(Equal(defaultReplicaCount))
		})

		It("Test reconciler to make no change to node count", func() {
			fakeCloudProvider.NodeReplicas[sng.Spec.ID] = defaultReplicaCount // set existing replicas equal to desired
			Expect(fakeController.Reconcile(sng)).To(Succeed())
			Expect(fakeCloudProvider.NodeReplicas[sng.Spec.ID]).To(Equal(defaultReplicaCount))
		})

		It("Scale up nodes when not node group is stabilized and check status condition", func() {
			Expect(fakeController.Reconcile(sng)).To(Succeed())
			Expect(fakeCloudProvider.NodeReplicas[sng.Spec.ID]).To(Equal(defaultReplicaCount))
			Expect(sng.StatusConditions().GetCondition(v1alpha1.Stabilized).IsTrue()).To(Equal(true))
			Expect(sng.StatusConditions().GetCondition(v1alpha1.Stabilized).Message).To(Equal(""))
		})

		It("Scale up nodes when not node group is NOT stabilized and check status condition", func() {
			var dummyMessage = "test message"
			fakeCloudProvider.NodeGroupStable = false
			fakeCloudProvider.NodeGroupMessage = dummyMessage
			Expect(fakeController.Reconcile(sng)).To(Succeed())
			Expect(fakeCloudProvider.NodeReplicas[sng.Spec.ID]).To(Equal(defaultReplicaCount))
			Expect(sng.StatusConditions().GetCondition(v1alpha1.Stabilized).IsFalse()).To(Equal(true))
			Expect(sng.StatusConditions().GetCondition(v1alpha1.Stabilized).Message).To(Equal(dummyMessage))
		})

		It("Retryable error while reconciling", func() {
			var dummyMessage = "test message"
			fakeCloudProvider.WantErr = fake.RetryableError(fmt.Errorf(dummyMessage)) // retryable error
			existingReplicas := fakeCloudProvider.NodeReplicas[sng.Spec.ID]
			Expect(fakeController.Reconcile(sng)).To(Succeed())
			Expect(fakeCloudProvider.NodeReplicas[sng.Spec.ID]).To(Equal(existingReplicas))
			Expect(sng.StatusConditions().GetCondition(v1alpha1.AbleToScale).IsFalse()).To(Equal(true))
			Expect(sng.StatusConditions().GetCondition(v1alpha1.AbleToScale).Message).To(Equal(dummyMessage))
		})

	})
})
