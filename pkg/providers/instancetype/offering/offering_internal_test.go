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

package offering

import (
	"context"
	"testing"
	"unique"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/arczonalshift"
)

// InjectOfferings rebuilds each instance type around its new offerings. Fields it doesn't own have to carry over, or
// anything populated before injection (such as a downstream provider's dynamic resource sets) would be dropped.
func TestInjectOfferingsKeepsDynamicResources(t *testing.T) {
	drSets := []*cloudprovider.DynamicResources{{
		Requirements: scheduling.NewRequirements(),
		ResourceSliceTemplates: []*cloudprovider.ResourceSliceTemplate{{
			Driver: unique.Make("gpu.example.com"),
			Pool:   cloudprovider.ResourcePool{Name: unique.Make("pool")},
		}},
	}}
	bindings := []*cloudprovider.AttributeBinding{{Attribute: "gpu.example.com/driverVersion"}}
	it := &cloudprovider.InstanceType{
		Name:              "g6.12xlarge",
		Requirements:      scheduling.NewRequirements(),
		DynamicResources:  drSets,
		AttributeBindings: bindings,
	}

	p := &DefaultProvider{zonalshiftProvider: arczonalshift.NewNoopProvider()}
	its := p.InjectOfferings(context.Background(), []*cloudprovider.InstanceType{it}, map[ec2types.InstanceType]ec2types.InstanceTypeInfo{}, &v1.EC2NodeClass{}, sets.New[string]())

	if len(its) != 1 {
		t.Fatalf("expected 1 instance type, got %d", len(its))
	}
	if its[0] == it {
		t.Fatalf("expected InjectOfferings to rebuild the instance type")
	}
	if len(its[0].DynamicResources) != 1 || its[0].DynamicResources[0] != drSets[0] {
		t.Errorf("expected DynamicResources to carry over, got %v", its[0].DynamicResources)
	}
	if len(its[0].AttributeBindings) != 1 || its[0].AttributeBindings[0] != bindings[0] {
		t.Errorf("expected AttributeBindings to carry over, got %v", its[0].AttributeBindings)
	}
}
