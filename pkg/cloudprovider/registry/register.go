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

package registry

import (
	"context"
	"fmt"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha4"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	v1 "k8s.io/api/core/v1"
)

func NewCloudProvider(ctx context.Context, options cloudprovider.Options) cloudprovider.CloudProvider {
	cloudProvider := newCloudProvider(ctx, options)
	RegisterOrDie(ctx, cloudProvider)
	return cloudProvider
}

// RegisterOrDie populates supported instance types, zones, operating systems,
// architectures, and validation logic. This operation should only be called
// once at startup time. Typically, this call is made by NewCloudProvider(), but
// must be called if the cloud provider is constructed manually (e.g. tests).
func RegisterOrDie(ctx context.Context, cloudProvider cloudprovider.CloudProvider) {
	zones := map[string]bool{}
	architectures := map[string]bool{}
	operatingSystems := map[string]bool{}

	instanceTypes, err := cloudProvider.GetInstanceTypes(ctx)
	if err != nil {
		panic(fmt.Sprintf("Failed to retrieve instance types, %s", err.Error()))
	}
	for _, instanceType := range instanceTypes {
		v1alpha4.WellKnownLabels[v1.LabelInstanceTypeStable] = append(v1alpha4.WellKnownLabels[v1.LabelInstanceTypeStable], instanceType.Name())
		architectures[instanceType.Architecture()] = true
		for _, zone := range instanceType.Zones() {
			zones[zone] = true
		}
		for _, operatingSystem := range instanceType.OperatingSystems() {
			operatingSystems[operatingSystem] = true
		}
	}
	for zone := range zones {
		v1alpha4.WellKnownLabels[v1.LabelTopologyZone] = append(v1alpha4.WellKnownLabels[v1.LabelTopologyZone], zone)
	}
	for architecture := range architectures {
		v1alpha4.WellKnownLabels[v1.LabelArchStable] = append(v1alpha4.WellKnownLabels[v1.LabelArchStable], architecture)
	}
	for operatingSystem := range operatingSystems {
		v1alpha4.WellKnownLabels[v1.LabelOSStable] = append(v1alpha4.WellKnownLabels[v1.LabelOSStable], operatingSystem)
	}

	v1alpha4.ValidateHook = cloudProvider.Validate
	v1alpha4.DefaultHook = cloudProvider.Default
	v1alpha4.ConstrainHook = cloudProvider.Constrain
}
