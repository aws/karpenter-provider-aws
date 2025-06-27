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

package fake

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/awslabs/operatorpkg/status"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	corecloudprovider "sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

const (
	DefaultRegion  = "us-west-2"
	DefaultAccount = "123456789"
)

var _ corecloudprovider.CloudProvider = (*CloudProvider)(nil)

type CloudProvider struct {
	InstanceTypes []*corecloudprovider.InstanceType
	ValidAMIs     []string
}

func (c *CloudProvider) Create(_ context.Context, _ *karpv1.NodeClaim) (*karpv1.NodeClaim, error) {
	name := test.RandomName()
	return &karpv1.NodeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: karpv1.NodeClaimStatus{
			ProviderID: RandomProviderID(),
		},
	}, nil
}

func (c *CloudProvider) GetInstanceTypes(_ context.Context, _ *karpv1.NodePool) ([]*corecloudprovider.InstanceType, error) {
	if c.InstanceTypes != nil {
		return c.InstanceTypes, nil
	}
	return []*corecloudprovider.InstanceType{
		{Name: "default-instance-type"},
	}, nil
}

func (c *CloudProvider) IsDrifted(_ context.Context, nodeClaim *karpv1.NodeClaim) (corecloudprovider.DriftReason, error) {
	return "drifted", nil
}

func (c *CloudProvider) Get(context.Context, string) (*karpv1.NodeClaim, error) {
	return nil, nil
}

func (c *CloudProvider) List(context.Context) ([]*karpv1.NodeClaim, error) {
	return nil, nil
}

func (c *CloudProvider) Delete(context.Context, *karpv1.NodeClaim) error {
	return nil
}

func (c *CloudProvider) DisruptionReasons() []karpv1.DisruptionReason {
	return nil
}

// Name returns the CloudProvider implementation name.
func (c *CloudProvider) Name() string {
	return "fake"
}

func (c *CloudProvider) GetSupportedNodeClasses() []status.Object {
	return []status.Object{&v1.EC2NodeClass{}}
}

func (c *CloudProvider) RepairPolicies() []corecloudprovider.RepairPolicy {
	return []corecloudprovider.RepairPolicy{}
}

// GenerateDefaultPriceOutput generates default output that can be set on the pricing provider
// if a test needs pricing data and is just using the default instance types
func GenerateDefaultPriceOutput() (*ec2.DescribeSpotPriceHistoryOutput, *pricing.GetProductsOutput) {
	var priceList []string
	odPrice := map[ec2types.InstanceType]float64{}
	for _, elem := range defaultDescribeInstanceTypesOutput.InstanceTypes {
		odPrice[elem.InstanceType] = float64(int64(lo.FromPtr[int32](elem.VCpuInfo.DefaultVCpus)) + lo.FromPtr[int64](elem.MemoryInfo.SizeInMiB))
		priceList = append(priceList, NewOnDemandPrice(string(elem.InstanceType), odPrice[elem.InstanceType]))
	}
	var spotPriceHistory []ec2types.SpotPrice
	for _, elem := range defaultDescribeInstanceTypeOfferingsOutput.InstanceTypeOfferings {
		spotPriceHistory = append(spotPriceHistory, ec2types.SpotPrice{
			InstanceType:     elem.InstanceType,
			AvailabilityZone: elem.Location,
			// Model spot pricing as 70% of OD pricing
			SpotPrice: lo.ToPtr(fmt.Sprint(odPrice[elem.InstanceType] * 0.7)),
			Timestamp: lo.ToPtr(time.Now()),
		})
	}
	return &ec2.DescribeSpotPriceHistoryOutput{SpotPriceHistory: spotPriceHistory}, &pricing.GetProductsOutput{PriceList: priceList}
}
