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

package node

import (
	"context"
	"fmt"

	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
)

type Initialization struct {
	kubeClient    client.Client
	cloudProvider cloudprovider.CloudProvider
}

// Reconcile reconciles the node
func (r *Initialization) Reconcile(ctx context.Context, provisioner *v1alpha5.Provisioner, n *v1.Node) (reconcile.Result, error) {
	// node has been previously determined to be ready, so there's nothing to do
	if n.Labels[v1alpha5.LabelNodeReady] == "true" {
		return reconcile.Result{}, nil
	}

	// node is not ready per the label, we need to check if kubelet indicates that the node is ready as well as if
	// startup taints are removed and extended resources have been initialized
	instanceType, err := r.getInstanceType(ctx, provisioner, n.Labels[v1.LabelInstanceTypeStable])
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("determining instance type, %w", err)
	}
	if !isReady(n, provisioner, instanceType) {
		return reconcile.Result{}, nil
	}

	n.Labels[v1alpha5.LabelNodeReady] = "true"
	return reconcile.Result{}, nil
}

func (r *Initialization) getInstanceType(ctx context.Context, provisioner *v1alpha5.Provisioner, instanceTypeName string) (cloudprovider.InstanceType, error) {
	instanceTypes, err := r.cloudProvider.GetInstanceTypes(ctx, provisioner.Spec.Provider)
	if err != nil {
		return nil, err
	}
	// The instance type may not be found which can occur if the instance type label was removed/edited.  This shouldn't occur,
	// but if it does we only lose the ability to check for extended resources.
	return lo.FindOrElse(instanceTypes, nil, func(it cloudprovider.InstanceType) bool { return it.Name() == instanceTypeName }), nil
}
