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

package instancetype_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype"
)

func TestInstanceCapabilityNestedVirtualizationLabel(t *testing.T) {
	ctx := options.ToContext(context.Background(), &options.Options{VMMemoryOverheadPercent: 0.075})
	instanceTypes, err := fake.NewEC2API().DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{})
	if err != nil {
		t.Fatalf("describing instance types: %v", err)
	}

	m7iFlex, ok := lo.Find(instanceTypes.InstanceTypes, func(info ec2types.InstanceTypeInfo) bool {
		return info.InstanceType == "m7i-flex.large"
	})
	if !ok {
		t.Fatal("expected m7i-flex.large in fake instance types")
	}
	if !lo.Contains(m7iFlex.ProcessorInfo.SupportedFeatures, ec2types.SupportedAdditionalProcessorFeatureNestedVirtualization) {
		t.Fatal("expected m7i-flex.large fake data to advertise nested virtualization support")
	}

	m5Large, ok := lo.Find(instanceTypes.InstanceTypes, func(info ec2types.InstanceTypeInfo) bool {
		return info.InstanceType == "m5.large"
	})
	if !ok {
		t.Fatal("expected m5.large in fake instance types")
	}

	itWithNestedVirt := instancetype.NewInstanceType(ctx, m7iFlex, fake.DefaultRegion, []string{"test-zone-1a"}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, v1.AMIFamilyAL2, nil)
	if !itWithNestedVirt.Requirements.Get(v1.LabelInstanceCapabilityNestedVirtualization).Has("true") {
		t.Fatalf("expected nested virtualization capability label true for %s", m7iFlex.InstanceType)
	}

	itWithoutNestedVirt := instancetype.NewInstanceType(ctx, m5Large, fake.DefaultRegion, []string{"test-zone-1a"}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, v1.AMIFamilyAL2, nil)
	if !itWithoutNestedVirt.Requirements.Get(v1.LabelInstanceCapabilityNestedVirtualization).Has("false") {
		t.Fatalf("expected nested virtualization capability label false for %s", m5Large.InstanceType)
	}
}
