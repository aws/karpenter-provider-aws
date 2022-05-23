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

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
)

func NewCloudProvider(ctx context.Context) cloudprovider.CloudProvider {
	cloudProvider := newCloudProvider(ctx)
	RegisterOrDie(cloudProvider)
	return cloudProvider
}

// RegisterOrDie populates supported instance types, zones, operating systems,
// architectures, and validation logic. This operation should only be called
// once at startup time. Typically, this call is made by NewCloudProvider(), but
// must be called if the cloud provider is constructed manually (e.g. tests).
func RegisterOrDie(cloudProvider cloudprovider.CloudProvider) {
	v1alpha5.ValidateHook = cloudProvider.Validate
	v1alpha5.DefaultHook = cloudProvider.Default
}
