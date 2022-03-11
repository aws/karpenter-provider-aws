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

	"k8s.io/apimachinery/pkg/version"

	"github.com/aws/karpenter/pkg/cloudprovider/aws/launchtemplate"
)

type K8sClient struct {
	Version launchtemplate.K8sVersion
}

func (c *K8sClient) ServerVersion(ctx context.Context) (*launchtemplate.K8sVersion, error) {
	return &c.Version, nil
}

func DefaultK8sClient() *K8sClient {
	return &K8sClient{
		Version: launchtemplate.K8sVersion{
			ServerVersion: version.Info{},
			Major:         1,
			Minor:         21,
		},
	}
}
