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

package cache_test

import (
	"context"
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	gocache "github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/cache"
	"github.com/aws/karpenter-provider-aws/pkg/test"
)

var ctx context.Context

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cache")
}

var _ = Describe("Cache", func() {
	var unavailableOfferingCache *cache.UnavailableOfferings
	var validationCache *cache.Validation

	BeforeEach(func() {
		unavailableOfferingCache = cache.NewUnavailableOfferings()
		validationCache = cache.NewValidation(gocache.New(cache.ValidationTTL, cache.DefaultCleanupInterval))
	})
	Context("Unavailable Offering Cache", func() {
		It("should mark offerings as unavailable when calling MarkUnavailable", func() {
			// offerings should initially not be marked as unavailable
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Large, "test-zone-1a", []string{}, karpv1.CapacityTypeOnDemand)).To(BeFalse())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1b", []string{}, karpv1.CapacityTypeSpot)).To(BeFalse())

			// m5.large on-demand should return that it's unavailable when we mark it
			unavailableOfferingCache.MarkUnavailable(ctx, ec2types.InstanceTypeM5Large, "test-zone-1a", karpv1.CapacityTypeOnDemand, map[string]string{"reason": "test"})
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Large, "test-zone-1a", []string{}, karpv1.CapacityTypeOnDemand)).To(BeTrue())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1b", []string{}, karpv1.CapacityTypeSpot)).To(BeFalse())

			// m5.xlarge shouldn't return that it's unavailable when marking an unrelated instance type
			unavailableOfferingCache.MarkUnavailable(ctx, ec2types.InstanceTypeM5Large, "test-zone-1b", karpv1.CapacityTypeOnDemand, map[string]string{"reason": "test"})
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Large, "test-zone-1a", []string{}, karpv1.CapacityTypeOnDemand)).To(BeTrue())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1b", []string{}, karpv1.CapacityTypeSpot)).To(BeFalse())

			// m5.xlarge spot should return that it's unavailable when we mark it
			unavailableOfferingCache.MarkUnavailable(ctx, ec2types.InstanceTypeM5Xlarge, "test-zone-1b", karpv1.CapacityTypeSpot, map[string]string{"reason": "test"})
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Large, "test-zone-1a", []string{}, karpv1.CapacityTypeOnDemand)).To(BeTrue())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1b", []string{}, karpv1.CapacityTypeSpot)).To(BeTrue())
		})
		It("should mark offerings as unavailable with fleet error reasons", func() {
			// offerings should initially not be marked as unavailable
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Large, "test-zone-1a", []string{}, karpv1.CapacityTypeOnDemand)).To(BeFalse())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1b", []string{}, karpv1.CapacityTypeSpot)).To(BeFalse())

			// m5.large on-demand should return that it's unavailable when we mark it
			unavailableOfferingCache.MarkUnavailable(ctx, ec2types.InstanceTypeM5Large, "test-zone-1a", karpv1.CapacityTypeOnDemand, map[string]string{
				"reason": "InsufficientInstanceCapacity",
			})
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Large, "test-zone-1a", []string{}, karpv1.CapacityTypeOnDemand)).To(BeTrue())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1b", []string{}, karpv1.CapacityTypeSpot)).To(BeFalse())

			// m5.xlarge shouldn't return that it's unavailable when marking an unrelated instance type
			unavailableOfferingCache.MarkUnavailable(ctx, ec2types.InstanceTypeM5Large, "test-zone-1b", karpv1.CapacityTypeOnDemand, map[string]string{
				"reason": "InsufficientInstanceCapacity",
			})
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Large, "test-zone-1a", []string{}, karpv1.CapacityTypeOnDemand)).To(BeTrue())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1b", []string{}, karpv1.CapacityTypeSpot)).To(BeFalse())

			// m5.xlarge spot should return that it's unavailable when we mark it
			unavailableOfferingCache.MarkUnavailable(ctx, ec2types.InstanceTypeM5Xlarge, "test-zone-1b", karpv1.CapacityTypeSpot, map[string]string{
				"reason": "InsufficientInstanceCapacity",
			})
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Large, "test-zone-1a", []string{}, karpv1.CapacityTypeOnDemand)).To(BeTrue())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1b", []string{}, karpv1.CapacityTypeSpot)).To(BeTrue())
		})
		It("should mark offerings as unavailable when calling MarkCapacityTypeUnavailable", func() {
			// offerings should initially not be marked as unavailable
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Large, "test-zone-1a", []string{}, karpv1.CapacityTypeOnDemand)).To(BeFalse())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1a", []string{}, karpv1.CapacityTypeOnDemand)).To(BeFalse())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1a", []string{}, karpv1.CapacityTypeSpot)).To(BeFalse())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1b", []string{}, karpv1.CapacityTypeSpot)).To(BeFalse())

			// mark all spot offerings as unavailable
			unavailableOfferingCache.MarkCapacityTypeUnavailable(karpv1.CapacityTypeSpot)
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Large, "test-zone-1a", []string{}, karpv1.CapacityTypeOnDemand)).To(BeFalse())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1a", []string{}, karpv1.CapacityTypeOnDemand)).To(BeFalse())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1a", []string{}, karpv1.CapacityTypeSpot)).To(BeTrue())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1b", []string{}, karpv1.CapacityTypeSpot)).To(BeTrue())

			// mark all on-demand offerings as unavailable
			unavailableOfferingCache.MarkCapacityTypeUnavailable(karpv1.CapacityTypeOnDemand)
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Large, "test-zone-1a", []string{}, karpv1.CapacityTypeOnDemand)).To(BeTrue())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1a", []string{}, karpv1.CapacityTypeOnDemand)).To(BeTrue())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1a", []string{}, karpv1.CapacityTypeSpot)).To(BeTrue())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1b", []string{}, karpv1.CapacityTypeSpot)).To(BeTrue())
		})
		It("should mark offerings as unavailable when calling MarkSubnetUnavailable", func() {
			zoneInfo := []v1.ZoneInfo{
				{
					Zone:      "test-zone-1a",
					ZoneID:    "tstz1-1a",
					SubnetIDs: []string{"subnet-test1"},
				},
				{
					Zone:      "test-zone-1b",
					ZoneID:    "tstz1-1b",
					SubnetIDs: []string{"subnet-test2"},
				},
				{
					Zone:      "test-zone-1c",
					ZoneID:    "tstz1-1c",
					SubnetIDs: []string{"subnet-test3"},
				},
			}

			// offerings should initially not be marked as unavailable
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Large, "test-zone-1a",
				test.GetSubnetsFromZone("test-zone-1a", zoneInfo), karpv1.CapacityTypeOnDemand)).To(BeFalse())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1a",
				test.GetSubnetsFromZone("test-zone-1a", zoneInfo), karpv1.CapacityTypeOnDemand)).To(BeFalse())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1a",
				test.GetSubnetsFromZone("test-zone-1a", zoneInfo), karpv1.CapacityTypeSpot)).To(BeFalse())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1b",
				test.GetSubnetsFromZone("test-zone-1b", zoneInfo), karpv1.CapacityTypeSpot)).To(BeFalse())

			// mark all test-zone-1a offerings as unavailable
			unavailableOfferingCache.MarkSubnetUnavailable("subnet-test1")
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Large, "test-zone-1a",
				test.GetSubnetsFromZone("test-zone-1a", zoneInfo), karpv1.CapacityTypeOnDemand)).To(BeTrue())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1a",
				test.GetSubnetsFromZone("test-zone-1a", zoneInfo), karpv1.CapacityTypeOnDemand)).To(BeTrue())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1a",
				test.GetSubnetsFromZone("test-zone-1a", zoneInfo), karpv1.CapacityTypeSpot)).To(BeTrue())
			Expect(unavailableOfferingCache.IsUnavailable(ec2types.InstanceTypeM5Xlarge, "test-zone-1b",
				test.GetSubnetsFromZone("test-zone-1b", zoneInfo), karpv1.CapacityTypeSpot)).To(BeFalse())
		})
		It("should increase sequence number when unavailability changes", func() {
			// sequence numbers should initially be 0
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Large)).To(BeNumerically("==", 0))
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Xlarge)).To(BeNumerically("==", 0))

			// marking m5.large as unavailable should increase the sequence number for that instance type but not others
			unavailableOfferingCache.MarkUnavailable(ctx, ec2types.InstanceTypeM5Large, "test-zone-1a", karpv1.CapacityTypeOnDemand, map[string]string{"reason": "test"})
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Large)).To(BeNumerically("==", 1))
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Xlarge)).To(BeNumerically("==", 0))

			// marking m5.xlarge as unavailable should increase the sequence number for that instance type but not others
			unavailableOfferingCache.MarkUnavailable(ctx, ec2types.InstanceTypeM5Xlarge, "test-zone-1a", karpv1.CapacityTypeOnDemand, map[string]string{"reason": "test"})
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Large)).To(BeNumerically("==", 1))
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Xlarge)).To(BeNumerically("==", 1))

			// marking test-zone-1a as unavailable should increase the sequence number for all instance types
			unavailableOfferingCache.MarkSubnetUnavailable("subnet-test1")
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Large)).To(BeNumerically("==", 2))
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Xlarge)).To(BeNumerically("==", 2))

			// marking test-zone-1b as unavailable should increase the sequence number for all instance types
			unavailableOfferingCache.MarkSubnetUnavailable("subnet-test2")
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Large)).To(BeNumerically("==", 3))
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Xlarge)).To(BeNumerically("==", 3))

			// marking on-demand capacity type as unavailable should increase the sequence number for all instance types
			unavailableOfferingCache.MarkCapacityTypeUnavailable(karpv1.CapacityTypeOnDemand)
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Large)).To(BeNumerically("==", 4))
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Xlarge)).To(BeNumerically("==", 4))

			// deleting m5.xlarge from the cache should increase the sequence number for that instance type but not others
			unavailableOfferingCache.Delete(ec2types.InstanceTypeM5Xlarge, "test-zone-1a", karpv1.CapacityTypeOnDemand)
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Large)).To(BeNumerically("==", 4))
			Expect(unavailableOfferingCache.SeqNum(ec2types.InstanceTypeM5Xlarge)).To(BeNumerically("==", 5))
		})
	})
	Context("Validation Cache", func() {
		var nodeClass *v1.EC2NodeClass
		var tags map[string]string
		BeforeEach(func() {
			nodeClass = test.EC2NodeClass()
			tags = map[string]string{"a": "b", "c": "d"}
		})
		DescribeTable("should return no cache hit when nodeclass fields change",
			func(changeNodeClass func(*v1.EC2NodeClass)) {
				validationCache.SetSuccess(nodeClass, tags)
				_, found := validationCache.Get(nodeClass, tags)
				Expect(found).To(BeTrue())
				changeNodeClass(nodeClass)
				_, found = validationCache.Get(nodeClass, tags)
				Expect(found).To(BeFalse())
			},
			Entry("nodeClass Subnets change", func(nc *v1.EC2NodeClass) {
				nc.Status.Subnets = []v1.Subnet{{ID: "1", Zone: "zone"}}
			}),
			Entry("nodeClass SecurityGroups change", func(nc *v1.EC2NodeClass) {
				nc.Status.SecurityGroups = []v1.SecurityGroup{{ID: "1"}}
			}),
			Entry("nodeClass AMIs change", func(nc *v1.EC2NodeClass) {
				nc.Status.AMIs = []v1.AMI{{ID: "1"}}
			}),
			Entry("nodeClass InstanceProfile changes", func(nc *v1.EC2NodeClass) {
				nc.Status.InstanceProfile = "profile"
			}),
			Entry("nodeClass MetadataOptionss change", func(nc *v1.EC2NodeClass) {
				nc.Spec.MetadataOptions = &v1.MetadataOptions{HTTPEndpoint: lo.ToPtr("end")}
			}),
			Entry("nodeClass BlockDeviceMappings change", func(nc *v1.EC2NodeClass) {
				nc.Spec.BlockDeviceMappings = []*v1.BlockDeviceMapping{{DeviceName: lo.ToPtr("name")}}
			}),
			Entry("nodeClass Kubelet changes", func(nc *v1.EC2NodeClass) {
				nc.Spec.Kubelet = test.MustMakeKubeletConfiguration(map[string]interface{}{"maxPods": 10})
			}),
		)
		It("should return no cache hit when tags change", func() {
			validationCache.SetFailure(nodeClass, tags, "failure", "failure message")
			_, found := validationCache.Get(nodeClass, tags)
			Expect(found).To(BeTrue())
			newTags := map[string]string{"a": "b"}
			_, found = validationCache.Get(nodeClass, newTags)
			Expect(found).To(BeFalse())
		})
		It("should ignore the nodepool tag when computing the cache key", func() {
			validationCache.SetSuccess(nodeClass, lo.Assign(tags, map[string]string{v1.NodePoolTagKey: ""}))
			entry, found := validationCache.Get(nodeClass, lo.Assign(tags, map[string]string{v1.NodePoolTagKey: "default"}))
			Expect(found).To(BeTrue())
			Expect(entry.Reason).To(BeEmpty())
		})
		It("should return the cached failure reason and message", func() {
			validationCache.SetFailure(nodeClass, tags, "reason", "message")
			entry, found := validationCache.Get(nodeClass, tags)
			Expect(found).To(BeTrue())
			Expect(entry).To(Equal(cache.ValidationEntry{Reason: "reason", Message: "message"}))
		})
		It("should clear cache entries for a specific nodeclass", func() {
			validationCache.SetSuccess(nodeClass, tags)
			validationCache.SetFailure(nodeClass, map[string]string{"a": "b"}, "failure", "failure message")
			nodeClassTwo := test.EC2NodeClass(v1.EC2NodeClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "different-nodeclass",
				},
			})
			validationCache.SetSuccess(nodeClassTwo, tags)
			Expect(len(validationCache.Items())).To(Equal(3))

			validationCache.ClearCacheEntries(nodeClass)
			Expect(len(validationCache.Items())).To(Equal(1))
			_, found := validationCache.Get(nodeClassTwo, tags)
			Expect(found).To(BeTrue())
		})
	})
})
