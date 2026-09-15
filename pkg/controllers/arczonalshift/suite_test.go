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

package arczonalshift_test

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	arcsdk "github.com/aws/aws-sdk-go-v2/service/arczonalshift"
	arczonalshifttypes "github.com/aws/aws-sdk-go-v2/service/arczonalshift/types"
	"github.com/awslabs/operatorpkg/object"
	"github.com/awslabs/operatorpkg/status"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/arczonalshift"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/test"
)

var ctx context.Context
var env *coretest.Environment
var awsEnv *test.Environment
var recorder *coretest.EventRecorder
var controller *arczonalshift.Controller
var nodeClass *v1.EC2NodeClass
var nodePool *karpv1.NodePool

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "ARCZonalShiftController")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(coretest.WithCRDs(apis.CRDs...), coretest.WithCRDs(v1alpha1.CRDs...))
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	awsEnv = test.NewEnvironment(ctx, env)
	recorder = coretest.NewEventRecorder()
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())

	// Fresh controller per spec so the previousShiftedZones transition state does not leak between tests.
	controller = arczonalshift.NewController(env.Client, recorder, awsEnv.ZonalShiftProvider)

	// Subnets supply the zone name <-> zone ID mapping the controller uses to translate shifts (keyed by zone
	// ID) into NodePool zone requirements (keyed by zone name).
	nodeClass = test.EC2NodeClass(v1.EC2NodeClass{
		Status: v1.EC2NodeClassStatus{
			Subnets: []v1.Subnet{
				{ID: "subnet-test1", Zone: "test-zone-1a", ZoneID: "tstz1-1a"},
				{ID: "subnet-test2", Zone: "test-zone-1b", ZoneID: "tstz1-1b"},
				{ID: "subnet-test3", Zone: "test-zone-1c", ZoneID: "tstz1-1c"},
			},
		},
	})
	nodeClass.StatusConditions().SetTrue(status.ConditionReady)
	nodePool = coretest.NodePool(karpv1.NodePool{
		Spec: karpv1.NodePoolSpec{
			Template: karpv1.NodeClaimTemplate{
				Spec: karpv1.NodeClaimTemplateSpec{
					NodeClassRef: &karpv1.NodeClassReference{
						Group: object.GVK(nodeClass).Group,
						Kind:  object.GVK(nodeClass).Kind,
						Name:  nodeClass.Name,
					},
				},
			},
		},
	})
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
	awsEnv.Reset()
	recorder.Reset()
})

// setShiftedZones configures the fake ARC API to report the given zone IDs as actively shifted away from.
func setShiftedZones(zoneIDs ...string) {
	shifts := lo.Map(zoneIDs, func(zoneID string, _ int) arczonalshifttypes.ZonalShiftInResource {
		return arczonalshifttypes.ZonalShiftInResource{
			AwayFrom:      aws.String(zoneID),
			ExpiryTime:    aws.Time(awsEnv.Clock.Now().Add(time.Hour)),
			AppliedStatus: arczonalshifttypes.AppliedStatusApplied,
		}
	})
	awsEnv.ARCZonalShiftAPI.GetManagedResourceBehavior.Output.Set(&arcsdk.GetManagedResourceOutput{ZonalShifts: shifts})
}

var _ = Describe("ARCZonalShiftController", func() {
	const detectedReason = "ZonalShiftActive"
	const clearedReason = "ZonalShiftCleared"

	It("should emit a Warning event on an affected NodePool when a zone shifts", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		setShiftedZones("tstz1-1a")
		ExpectSingletonReconciled(ctx, controller)

		Expect(recorder.Calls(detectedReason)).To(Equal(1))
		Expect(recorder.DetectedEvent("Zonal shift detected: offerings in zone test-zone-1a (tstz1-1a) are unavailable for this NodePool")).To(BeTrue())
	})

	It("should emit a Normal event on an affected NodePool when a zone shift clears", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		setShiftedZones("tstz1-1a")
		ExpectSingletonReconciled(ctx, controller)
		Expect(recorder.Calls(detectedReason)).To(Equal(1))

		setShiftedZones()
		ExpectSingletonReconciled(ctx, controller)

		Expect(recorder.Calls(clearedReason)).To(Equal(1))
		Expect(recorder.DetectedEvent("Zonal shift cleared: offerings in zone test-zone-1a (tstz1-1a) are restored for this NodePool")).To(BeTrue())
	})

	It("should not emit an event on a NodePool restricted to different zones", func() {
		nodePool.Spec.Template.Spec.Requirements = []karpv1.NodeSelectorRequirementWithMinValues{
			{
				Key:      corev1.LabelTopologyZone,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{"test-zone-1b", "test-zone-1c"},
			},
		}
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		setShiftedZones("tstz1-1a")
		ExpectSingletonReconciled(ctx, controller)

		Expect(recorder.Calls(detectedReason)).To(Equal(0))
	})

	It("should emit an event on a NodePool with no zone restriction", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		setShiftedZones("tstz1-1a")
		ExpectSingletonReconciled(ctx, controller)

		Expect(recorder.Calls(detectedReason)).To(Equal(1))
	})

	It("should not emit an event for a shift in a zone the NodeClass does not resolve to", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		// nodePool has no zone restriction, but the NodeClass has no subnet in tstz1-1z.
		setShiftedZones("tstz1-1z")
		ExpectSingletonReconciled(ctx, controller)

		Expect(recorder.Calls(detectedReason)).To(Equal(0))
	})

	It("should not emit events when there is no state change between reconciles", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		setShiftedZones("tstz1-1a")
		ExpectSingletonReconciled(ctx, controller)
		Expect(recorder.Calls(detectedReason)).To(Equal(1))

		recorder.Reset()
		ExpectSingletonReconciled(ctx, controller)
		Expect(recorder.Calls(detectedReason)).To(Equal(0))
		Expect(recorder.Calls(clearedReason)).To(Equal(0))
	})

	It("should only emit on NodePools affected by the shifted zone", func() {
		restricted := coretest.NodePool(karpv1.NodePool{
			Spec: karpv1.NodePoolSpec{
				Template: karpv1.NodeClaimTemplate{
					Spec: karpv1.NodeClaimTemplateSpec{
						NodeClassRef: &karpv1.NodeClassReference{
							Group: object.GVK(nodeClass).Group,
							Kind:  object.GVK(nodeClass).Kind,
							Name:  nodeClass.Name,
						},
						Requirements: []karpv1.NodeSelectorRequirementWithMinValues{
							{
								Key:      corev1.LabelTopologyZone,
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"test-zone-1b"},
							},
						},
					},
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, restricted, nodeClass)
		setShiftedZones("tstz1-1a")
		ExpectSingletonReconciled(ctx, controller)

		Expect(recorder.Calls(detectedReason)).To(Equal(1))
	})
})
