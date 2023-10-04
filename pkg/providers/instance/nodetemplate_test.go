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

package instance_test

import (
	"github.com/aws/aws-sdk-go/aws"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	corecloudprovider "github.com/aws/karpenter-core/pkg/cloudprovider"
	coretest "github.com/aws/karpenter-core/pkg/test"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
	nodeclaimutil "github.com/aws/karpenter-core/pkg/utils/nodeclaim"
	nodepoolutil "github.com/aws/karpenter-core/pkg/utils/nodepool"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/fake"
	"github.com/aws/karpenter/pkg/test"
	nodeclassutil "github.com/aws/karpenter/pkg/utils/nodeclass"
)

var _ = Describe("NodeTemplate/InstanceProvider", func() {
	var nodeTemplate *v1alpha1.AWSNodeTemplate
	var provisioner *v1alpha5.Provisioner
	var machine *v1alpha5.Machine
	BeforeEach(func() {
		nodeTemplate = &v1alpha1.AWSNodeTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name: coretest.RandomName(),
			},
			Spec: v1alpha1.AWSNodeTemplateSpec{
				AWS: v1alpha1.AWS{
					AMIFamily:             aws.String(v1alpha1.AMIFamilyAL2),
					SubnetSelector:        map[string]string{"*": "*"},
					SecurityGroupSelector: map[string]string{"*": "*"},
				},
			},
		}
		provisioner = test.Provisioner(coretest.ProvisionerOptions{
			Requirements: []v1.NodeSelectorRequirement{{
				Key:      v1alpha1.LabelInstanceCategory,
				Operator: v1.NodeSelectorOpExists,
			}},
			ProviderRef: &v1alpha5.MachineTemplateRef{
				APIVersion: nodeTemplate.APIVersion,
				Kind:       nodeTemplate.Kind,
				Name:       nodeTemplate.Name,
			},
		})
		machine = coretest.Machine(v1alpha5.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: provisioner.Name,
				},
			},
			Spec: v1alpha5.MachineSpec{
				MachineTemplateRef: &v1alpha5.MachineTemplateRef{
					Name: nodeTemplate.Name,
				},
			},
		})
	})
	It("should return an ICE error when all attempted instance types return an ICE error", func() {
		ExpectApplied(ctx, env.Client, machine, provisioner, nodeTemplate)
		awsEnv.EC2API.InsufficientCapacityPools.Set([]fake.CapacityPool{
			{CapacityType: v1alpha5.CapacityTypeOnDemand, InstanceType: "m5.xlarge", Zone: "test-zone-1a"},
			{CapacityType: v1alpha5.CapacityTypeOnDemand, InstanceType: "m5.xlarge", Zone: "test-zone-1b"},
			{CapacityType: v1alpha5.CapacityTypeSpot, InstanceType: "m5.xlarge", Zone: "test-zone-1a"},
			{CapacityType: v1alpha5.CapacityTypeSpot, InstanceType: "m5.xlarge", Zone: "test-zone-1b"},
		})
		instanceTypes, err := cloudProvider.GetInstanceTypes(ctx, nodepoolutil.New(provisioner))
		Expect(err).ToNot(HaveOccurred())

		// Filter down to a single instance type
		instanceTypes = lo.Filter(instanceTypes, func(i *corecloudprovider.InstanceType, _ int) bool { return i.Name == "m5.xlarge" })

		// Since all the capacity pools are ICEd. This should return back an ICE error
		instance, err := awsEnv.InstanceProvider.Create(ctx, nodeclassutil.New(nodeTemplate), nodeclaimutil.New(machine), instanceTypes)
		Expect(corecloudprovider.IsInsufficientCapacityError(err)).To(BeTrue())
		Expect(instance).To(BeNil())
	})
})
