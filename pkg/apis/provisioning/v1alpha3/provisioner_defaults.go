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

package v1alpha3

import (
	"context"
	"encoding/base64"

	"github.com/awslabs/karpenter/pkg/utils/filesystem"
	"github.com/spf13/afero"
)

const (
	// InClusterCABundlePath is normally available to Pods such as
	// Karpenter, as described here:
	// https://kubernetes.io/docs/tasks/access-application-cluster/access-cluster/#accessing-the-api-from-a-pod
	InClusterCABundlePath = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

// SetDefaults for the provisioner
func (p *Provisioner) SetDefaults(ctx context.Context) {}

// SetDefaults for the provisioner, cascading to all subspecs
//func (s *ProvisionerSpec) SetDefaults(ctx context.Context) {}

func (c *Cluster) GetCABundle(ctx context.Context) (*string, error) {
	if c.CABundle != nil {
		// If CABundle is explicitly provided, use that one. An empty
		// string is a valid value here if the intention is to disable
		// the in-cluster CABundle, and using the HTTP client's
		// default trust-store (CABundle) instead.
		return c.CABundle, nil
	}

	// Otherwise, fall back to the in-cluster configuration.
	fs := filesystem.For(ctx)
	exists, err := afero.Exists(fs, InClusterCABundlePath)
	if err != nil || !exists {
		return nil, err
	}
	binary, err := afero.ReadFile(filesystem.For(ctx), InClusterCABundlePath)
	if err != nil {
		return nil, err
	}
	encoded := base64.StdEncoding.EncodeToString(binary)
	return &encoded, nil
}
