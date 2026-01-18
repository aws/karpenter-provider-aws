/*
Copyright The Kubernetes Authors.

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

package overlay

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/controllers/nodeoverlay"
	"sigs.k8s.io/karpenter/pkg/operator/options"
)

type decorator struct {
	cloudprovider.CloudProvider
	kubeClient client.Client
	store      *nodeoverlay.InstanceTypeStore
}

// Decorate returns a new `CloudProvider` instance that will delegate the GetInstanceTypes
// calls to the argument, `cloudProvider`, and provide instance types with NodeOverlays applied to them. The
func Decorate(cloudProvider cloudprovider.CloudProvider, kubeClient client.Client, store *nodeoverlay.InstanceTypeStore) cloudprovider.CloudProvider {
	return &decorator{CloudProvider: cloudProvider, kubeClient: kubeClient, store: store}
}

func (d *decorator) GetInstanceTypes(ctx context.Context, nodePool *v1.NodePool) ([]*cloudprovider.InstanceType, error) {
	its, err := d.CloudProvider.GetInstanceTypes(ctx, nodePool)
	if err != nil {
		return []*cloudprovider.InstanceType{}, err
	}
	if options.FromContext(ctx).FeatureGates.NodeOverlay {
		its, err = d.store.ApplyAll(nodePool.Name, its)
		if err != nil {
			return []*cloudprovider.InstanceType{}, err
		}
	}
	return its, nil
}
