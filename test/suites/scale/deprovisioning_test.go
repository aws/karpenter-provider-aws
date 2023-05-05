package scale_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	awstest "github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/test/pkg/debug"
)

var _ = Describe("Deprovisioning", Label(debug.NoWatch), Label(debug.NoEvents), func() {
	var provisioner *v1alpha5.Provisioner
	var nodeTemplate *v1alpha1.AWSNodeTemplate
	var deployment *appsv1.Deployment
	var selector labels.Selector
	var dsCount int


	BeforeEach(func() {
		env.ExpectSettingsOverridden(map[string]string{
			"featureGates.driftEnabled": "true",
		})
		nodeTemplate = awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})
		provisioner = test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.MachineTemplateRef{
				Name: nodeTemplate.Name,
			},
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha1.LabelInstanceSize,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"4xlarge"},
				},
				{
					Key: v1alpha5.LabelCapacityType,
					Operator: v1.NodeSelectorOpIn,
					Values: []string{v1alpha1.CapacityTypeOnDemand},
				},
			},
			// No limits!!!
			// https://tenor.com/view/chaos-gif-22919457
			Limits: v1.ResourceList{},
		})
		deployment = test.Deployment()
		selector = labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels)
		dsCount = env.GetDaemonSetCount(provisioner)

	})

	AfterEach(func() {
		env.Cleanup()
	})

	AfterEach(func() {
		env.ExpectCleanCluster()
	})

	Context("Multiple Deprovisioners", func() {
		It("should be a dummy test to compile", func() {
			Skip("skip dummy test that isn't ready yet")
			// TODO @joinnis: Write the rest of this test case

			env.ExpectCreated(provisioner, nodeTemplate, deployment)
			fmt.Println(selector)
		})
	})
	Context("Consolidation", func() {})
	It("should deprovision all nodes when empty", func() {
		// Before Deprovisioning, we need to Provision the cluster to the state that we need.
		replicasPerNode := 1
		maxPodDensity := replicasPerNode + dsCount
		expectedNodeCount := 30
		replicas := replicasPerNode * expectedNodeCount

		deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
		provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
			MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
		}

		By("waiting for the deployment to deploy all of its pods")
		env.ExpectCreated(deployment)
		env.EventuallyExpectPendingPodCount(selector, replicas)

		By("kicking off provisioning by applying the provisioner and nodeTemplate")
		env.ExpectCreated(provisioner, nodeTemplate)

		env.EventuallyExpectCreatedMachineCount(">=", expectedNodeCount)
		env.EventuallyExpectCreatedNodeCount(">=", expectedNodeCount)
		env.EventuallyExpectInitializedNodeCount(">=", expectedNodeCount)
		env.EventuallyExpectHealthyPodCount(selector, replicas)

		createdNodes := env.Monitor.CreatedNodeCount()

		By(fmt.Sprintf("Created %d nodes. Resetting monitor for deprovisioning.", createdNodes))
		env.Monitor.Reset()
		By("waiting for all deployment pods to be deleted")
		// Fully scale down all pods to make nodes empty
		deployment.Spec.Replicas = lo.ToPtr[int32](0)
		env.ExpectDeleted(deployment)
		env.EventuallyExpectHealthyPodCount(selector, 0)

		By("kicking off deprovisioning by adding ttlSecondsAfterEmpty")
		provisioner.Spec.TTLSecondsAfterEmpty = lo.ToPtr[int64](0)
		env.ExpectCreatedOrUpdated(provisioner)

		env.EventuallyExpectDeletedNodeCount("==", createdNodes)
	})
	It("should expire all nodes", func () {
		// Before Deprovisioning, we need to Provision the cluster to the state that we need.
		replicasPerNode := 1
		maxPodDensity := replicasPerNode + dsCount
		expectedNodeCount := 30
		replicas := replicasPerNode * expectedNodeCount

		deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
		provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
			MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
		}

		By("waiting for the deployment to deploy all of its pods")
		env.ExpectCreated(deployment)
		env.EventuallyExpectPendingPodCount(selector, replicas)

		By("kicking off provisioning by applying the provisioner and nodeTemplate")
		env.ExpectCreated(provisioner, nodeTemplate)

		env.EventuallyExpectCreatedMachineCount(">=", expectedNodeCount)
		env.EventuallyExpectCreatedNodeCount(">=", expectedNodeCount)
		env.EventuallyExpectInitializedNodeCount(">=", expectedNodeCount)
		env.EventuallyExpectHealthyPodCount(selector, replicas)

		createdNodes := env.Monitor.CreatedNodeCount()

		By(fmt.Sprintf("Created %d nodes. Resetting monitor for deprovisioning.", createdNodes))
		env.Monitor.Reset()
		By("kicking off deprovisioning by adding expiration and another provisioner")
		// Change Provisioner limits so that replacement nodes will use another provisioner.
		provisioner.Spec.Limits = &v1alpha5.Limits{
			Resources: v1.ResourceList{
				v1.ResourceCPU: resource.MustParse("0"),
				v1.ResourceMemory: resource.MustParse("0Gi"),
			},
		}
		// Enable Expiration
		provisioner.Spec.TTLSecondsUntilExpired = lo.ToPtr[int64](0)

		noExpireProvisioner := test.Provisioner(test.ProvisionerOptions{
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha1.LabelInstanceSize,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"4xlarge"},
				},
				{
					Key: v1alpha5.LabelCapacityType,
					Operator: v1.NodeSelectorOpIn,
					Values: []string{v1alpha1.CapacityTypeOnDemand},
				},
			},
			ProviderRef: &v1alpha5.MachineTemplateRef{
				Name: nodeTemplate.Name,
			},
		})
		env.ExpectCreatedOrUpdated(provisioner, noExpireProvisioner)
		env.EventuallyExpectDeletedNodeCount("==", createdNodes)
	})
	It("should drift all nodes", func () {
		// Before Deprovisioning, we need to Provision the cluster to the state that we need.
		replicasPerNode := 1
		maxPodDensity := replicasPerNode + dsCount
		expectedNodeCount := 30
		replicas := replicasPerNode * expectedNodeCount

		deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
		provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
			MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
		}

		By("waiting for the deployment to deploy all of its pods")
		env.ExpectCreated(deployment)
		env.EventuallyExpectPendingPodCount(selector, replicas)

		By("kicking off provisioning by applying the provisioner and nodeTemplate")
		env.ExpectCreated(provisioner, nodeTemplate)

		env.EventuallyExpectCreatedMachineCount(">=", expectedNodeCount)
		env.EventuallyExpectCreatedNodeCount(">=", expectedNodeCount)
		env.EventuallyExpectInitializedNodeCount(">=", expectedNodeCount)
		env.EventuallyExpectHealthyPodCount(selector, replicas)

		// Fully scale down all pods to make nodes empty
		deployment.Spec.Replicas = lo.ToPtr[int32](0)
		env.ExpectCreatedOrUpdated(deployment)
		env.EventuallyExpectHealthyPodCount(selector, 0)

		provisioner.Spec.TTLSecondsAfterEmpty = lo.ToPtr[int64](0)
		env.ExpectCreatedOrUpdated(deployment)

		env.EventuallyExpectDeletedNodeCount("==", expectedNodeCount)
	})
	Context("Expiration", func() {
		// Before Deprovisioning, we need to Provision the cluster to the state that we need.
		replicas := 6000
		maxPodDensity := 200
		expectedNodeCount := 30

		deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
		provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
			MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
		}

		By("waiting for the deployment to deploy all of its pods")
		env.ExpectCreated(deployment)
		env.EventuallyExpectPendingPodCount(selector, replicas)

		By("kicking off provisioning by applying the provisioner and nodeTemplate")
		env.ExpectCreated(provisioner, nodeTemplate)

		env.EventuallyExpectCreatedMachineCount(">=", expectedNodeCount)
		env.EventuallyExpectCreatedNodeCount(">=", expectedNodeCount)
		env.EventuallyExpectInitializedNodeCount(">=", expectedNodeCount)
		env.EventuallyExpectHealthyPodCount(selector, replicas)

		// Change Provisioner limits so that replacement nodes will use another provisioner.
		provisioner.Spec.Limits = &v1alpha5.Limits{
			Resources: v1.ResourceList{
				v1.ResourceCPU: resource.MustParse("0"),
				v1.ResourceMemory: resource.MustParse("0Gi"),
			},
		}
		// Enable Expiration
		provisioner.Spec.TTLSecondsUntilExpired = lo.ToPtr[int64](0)

		noExpireProvisioner := test.Provisioner(test.ProvisionerOptions{
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha1.LabelInstanceSize,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"4xlarge"},
				},
				{
					Key: v1alpha5.LabelCapacityType,
					Operator: v1.NodeSelectorOpIn,
					Values: []string{v1alpha1.CapacityTypeOnDemand},
				},
			},
			ProviderRef: &v1alpha5.MachineTemplateRef{
				Name: nodeTemplate.Name,
			},
		})
		env.ExpectCreatedOrUpdated(provisioner, noExpireProvisioner)
		env.EventuallyExpectDeletedNodeCount("==", expectedNodeCount)
	})
	Context("Drift", func() {
		// Before Deprovisioning, we need to Provision the cluster to the state that we need.
		replicas := 6000
		maxPodDensity := 200
		expectedNodeCount := 30

		deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
		provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
			MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
		}
		provisioner.Spec.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1alpha1.LabelInstanceSize,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"4xlarge"},
			},
			{
				Key: v1alpha5.LabelCapacityType,
				Operator: v1.NodeSelectorOpIn,
				Values: []string{v1alpha1.CapacityTypeOnDemand},
			},
		}

		By("waiting for the deployment to deploy all of its pods")
		env.ExpectCreated(deployment)
		env.EventuallyExpectPendingPodCount(selector, replicas)

		By("kicking off provisioning by applying the provisioner and nodeTemplate")
		env.ExpectCreated(provisioner, nodeTemplate)

		env.EventuallyExpectCreatedMachineCount(">=", expectedNodeCount)
		env.EventuallyExpectCreatedNodeCount(">=", expectedNodeCount)
		env.EventuallyExpectInitializedNodeCount(">=", expectedNodeCount)
		env.EventuallyExpectHealthyPodCount(selector, replicas)

		createdNodes := env.Monitor.CreatedNodeCount()

		By(fmt.Sprintf("Created %d nodes. Resetting monitor for deprovisioning.", createdNodes))
		env.Monitor.Reset()
		By("kicking off deprovisioning for drift by changing the node template")
		// Change AMI Family to drift all nodes
		nodeTemplate.Spec.AMIFamily = lo.ToPtr("Bottlerocket")

		env.ExpectCreatedOrUpdated(nodeTemplate)
		env.EventuallyExpectDeletedNodeCount("==", createdNodes)
	})
	Context("Interruption", func() {})
})
