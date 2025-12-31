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

package hash_test

import (
	"context"
	"testing"

	"github.com/samber/lo"

	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	"github.com/awslabs/operatorpkg/object"
	"github.com/imdario/mergo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclass/hash"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var env *coretest.Environment
var awsEnv *test.Environment
var hashController *hash.Controller

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "EC2NodeClass")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(coretest.WithCRDs(apis.CRDs...), coretest.WithCRDs(v1alpha1.CRDs...), coretest.WithFieldIndexers(coretest.NodeClaimNodeClassRefFieldIndexer(ctx)))
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
	ctx = options.ToContext(ctx, test.Options())
	awsEnv = test.NewEnvironment(ctx, env)

	hashController = hash.NewController(env.Client, "us-west-2")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
	awsEnv.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("NodeClass Hash Controller", func() {
	var nodeClass *v1.EC2NodeClass
	var nodePool *karpv1.NodePool
	BeforeEach(func() {
		nodeClass = test.EC2NodeClass(v1.EC2NodeClass{
			Spec: v1.EC2NodeClassSpec{
				SubnetSelectorTerms: []v1.SubnetSelectorTerm{
					{
						Tags: map[string]string{"*": "*"},
					},
				},
				SecurityGroupSelectorTerms: []v1.SecurityGroupSelectorTerm{
					{
						Tags: map[string]string{"*": "*"},
					},
				},
				AMIFamily: lo.ToPtr(v1.AMIFamilyCustom),
				AMISelectorTerms: []v1.AMISelectorTerm{
					{
						Tags: map[string]string{"*": "*"},
					},
				},
			},
		})
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
	DescribeTable("should update the drift hash when static field is updated", func(changes *v1.EC2NodeClass) {
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, hashController, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		expectedHash := nodeClass.HashForRegion("us-west-2")
		Expect(nodeClass.ObjectMeta.Annotations[v1.AnnotationEC2NodeClassHash]).To(Equal(expectedHash))

		Expect(mergo.Merge(nodeClass, changes, mergo.WithOverride)).To(Succeed())

		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, hashController, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		expectedHashTwo := nodeClass.HashForRegion("us-west-2")
		Expect(nodeClass.Annotations[v1.AnnotationEC2NodeClassHash]).To(Equal(expectedHashTwo))
		Expect(expectedHash).ToNot(Equal(expectedHashTwo))

	},
		Entry("UserData Drift", &v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{UserData: aws.String("userdata-test-2")}}),
		Entry("Tags Drift", &v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{Tags: map[string]string{"keyTag-test-3": "valueTag-test-3"}}}),
		Entry("BlockDeviceMappings Drift", &v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{DeviceName: aws.String("map-device-test-3")}}}}),
		Entry("DetailedMonitoring Drift", &v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{DetailedMonitoring: aws.Bool(true)}}),
		Entry("MetadataOptions Drift", &v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{MetadataOptions: &v1.MetadataOptions{HTTPEndpoint: aws.String("disabled")}}}),
		Entry("Context Drift", &v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{Context: aws.String("context-2")}}),
	)
	It("should not update the drift hash when dynamic field is updated", func() {
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, hashController, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		expectedHash := nodeClass.HashForRegion("us-west-2")
		Expect(nodeClass.Annotations[v1.AnnotationEC2NodeClassHash]).To(Equal(expectedHash))

		nodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
			{
				ID: "subnet-test1",
			},
		}
		nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
			{
				ID: "sg-test1",
			},
		}
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
			{
				Tags: map[string]string{"ami-test-key": "ami-test-value"},
			},
		}

		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, hashController, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.Annotations[v1.AnnotationEC2NodeClassHash]).To(Equal(expectedHash))
	})
	It("should update ec2nodeclass-hash-version annotation when the ec2nodeclass-hash-version on the NodeClass does not match with the controller hash version", func() {
		nodeClass.Annotations = map[string]string{
			v1.AnnotationEC2NodeClassHash:        "abceduefed",
			v1.AnnotationEC2NodeClassHashVersion: "test",
		}
		ExpectApplied(ctx, env.Client, nodeClass)

		ExpectObjectReconciled(ctx, env.Client, hashController, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		expectedHash := nodeClass.HashForRegion("us-west-2")
		// Expect ec2nodeclass-hash on the NodeClass to be updated
		Expect(nodeClass.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHash, expectedHash))
		Expect(nodeClass.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHashVersion, v1.EC2NodeClassHashVersion))
	})
	It("should update ec2nodeclass-hash-versions on all NodeClaims when the ec2nodeclass-hash-version does not match with the controller hash version", func() {
		nodeClass.Annotations = map[string]string{
			v1.AnnotationEC2NodeClassHash:        "abceduefed",
			v1.AnnotationEC2NodeClassHashVersion: "test",
		}
		nodeClaimOne := coretest.NodeClaim(karpv1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{karpv1.NodePoolLabelKey: nodePool.Name},
				Annotations: map[string]string{
					v1.AnnotationEC2NodeClassHash:        "123456",
					v1.AnnotationEC2NodeClassHashVersion: "test",
				},
			},
			Spec: karpv1.NodeClaimSpec{
				NodeClassRef: &karpv1.NodeClassReference{
					Group: object.GVK(nodeClass).Group,
					Kind:  object.GVK(nodeClass).Kind,
					Name:  nodeClass.Name,
				},
			},
		})
		nodeClaimTwo := coretest.NodeClaim(karpv1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{karpv1.NodePoolLabelKey: nodePool.Name},
				Annotations: map[string]string{
					v1.AnnotationEC2NodeClassHash:        "123456",
					v1.AnnotationEC2NodeClassHashVersion: "test",
				},
			},
			Spec: karpv1.NodeClaimSpec{
				NodeClassRef: &karpv1.NodeClassReference{
					Group: object.GVK(nodeClass).Group,
					Kind:  object.GVK(nodeClass).Kind,
					Name:  nodeClass.Name,
				},
			},
		})

		ExpectApplied(ctx, env.Client, nodeClass, nodeClaimOne, nodeClaimTwo, nodePool)

		ExpectObjectReconciled(ctx, env.Client, hashController, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		nodeClaimOne = ExpectExists(ctx, env.Client, nodeClaimOne)
		nodeClaimTwo = ExpectExists(ctx, env.Client, nodeClaimTwo)

		expectedHash := nodeClass.HashForRegion("us-west-2")
		// Expect ec2nodeclass-hash on the NodeClaims to be updated
		Expect(nodeClaimOne.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHash, expectedHash))
		Expect(nodeClaimOne.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHashVersion, v1.EC2NodeClassHashVersion))
		Expect(nodeClaimTwo.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHash, expectedHash))
		Expect(nodeClaimTwo.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHashVersion, v1.EC2NodeClassHashVersion))
	})
	It("should not update ec2nodeclass-hash on all NodeClaims when the ec2nodeclass-hash-version matches the controller hash version", func() {
		nodeClass.Annotations = map[string]string{
			v1.AnnotationEC2NodeClassHash:        "abceduefed",
			v1.AnnotationEC2NodeClassHashVersion: "test-version",
		}
		nodeClaim := coretest.NodeClaim(karpv1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{karpv1.NodePoolLabelKey: nodePool.Name},
				Annotations: map[string]string{
					v1.AnnotationEC2NodeClassHash:        "1234564654",
					v1.AnnotationEC2NodeClassHashVersion: v1.EC2NodeClassHashVersion,
				},
			},
			Spec: karpv1.NodeClaimSpec{
				NodeClassRef: &karpv1.NodeClassReference{
					Group: object.GVK(nodeClass).Group,
					Kind:  object.GVK(nodeClass).Kind,
					Name:  nodeClass.Name,
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodeClass, nodeClaim, nodePool)

		ExpectObjectReconciled(ctx, env.Client, hashController, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

		expectedHash := nodeClass.HashForRegion("us-west-2")

		// Expect ec2nodeclass-hash on the NodeClass to be updated
		Expect(nodeClass.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHash, expectedHash))
		Expect(nodeClass.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHashVersion, v1.EC2NodeClassHashVersion))
		// Expect ec2nodeclass-hash on the NodeClaims to stay the same
		Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHash, "1234564654"))
		Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHashVersion, v1.EC2NodeClassHashVersion))
	})
	It("should not update ec2nodeclass-hash on the NodeClaim if it's drifted and the ec2nodeclass-hash-version does not match the controller hash version", func() {
		nodeClass.Annotations = map[string]string{
			v1.AnnotationEC2NodeClassHash:        "abceduefed",
			v1.AnnotationEC2NodeClassHashVersion: "test",
		}
		nodeClaim := coretest.NodeClaim(karpv1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{karpv1.NodePoolLabelKey: nodePool.Name},
				Annotations: map[string]string{
					v1.AnnotationEC2NodeClassHash:        "123456",
					v1.AnnotationEC2NodeClassHashVersion: "test",
				},
			},
			Spec: karpv1.NodeClaimSpec{
				NodeClassRef: &karpv1.NodeClassReference{
					Group: object.GVK(nodeClass).Group,
					Kind:  object.GVK(nodeClass).Kind,
					Name:  nodeClass.Name,
				},
			},
		})
		nodeClaim.StatusConditions().SetTrue(karpv1.ConditionTypeDrifted)
		ExpectApplied(ctx, env.Client, nodeClass, nodeClaim, nodePool)

		ExpectObjectReconciled(ctx, env.Client, hashController, nodeClass)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

		// Expect ec2nodeclass-hash on the NodeClaims to stay the same
		Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHash, "123456"))
		Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHashVersion, v1.EC2NodeClassHashVersion))
	})
})
