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

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"knative.dev/pkg/apis"
)

// RegisterOrDie populates supported instance types, zones, operating systems,
// architectures, and validation logic. This operation should only be called
// once at startup time. Typically, this call is made by NewCloudProvider(), but
// must be called if the cloud provider is constructed manually (e.g. tests).
func RegisterOrDie(api cloudprovider.API) {
	zones := map[string]bool{}
	architectures := map[string]bool{}
	operatingSystems := map[string]bool{}

	instanceTypes, err := api.GetInstanceTypes(context.Background())
	if err != nil {
		panic(fmt.Sprintf("Failed to retrieve instance types, %s", err.Error()))
	}
	for _, instanceType := range instanceTypes {
		v1alpha1.SupportedInstanceTypes = append(v1alpha1.SupportedInstanceTypes, instanceType.Name())
		for _, zone := range instanceType.Zones() {
			zones[zone] = true
		}
		for _, architecture := range instanceType.Architectures() {
			architectures[architecture] = true
		}
		for _, operatingSystem := range instanceType.OperatingSystems() {
			operatingSystems[operatingSystem] = true
		}
	}
	for zone := range zones {
		v1alpha1.SupportedZones = append(v1alpha1.SupportedZones, zone)
	}
	for architecture := range architectures {
		v1alpha1.SupportedArchitectures = append(v1alpha1.SupportedArchitectures, architecture)
	}
	for operatingSystem := range operatingSystems {
		v1alpha1.SupportedOperatingSystems = append(v1alpha1.SupportedOperatingSystems, operatingSystem)
	}
	v1alpha1.ValidationHook = func(ctx context.Context, spec *v1alpha1.ProvisionerSpec) *apis.FieldError {
		return api.Validate(ctx, spec)
	}
}
