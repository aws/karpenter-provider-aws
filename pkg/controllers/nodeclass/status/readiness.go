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

package status

import (
	"context"
	"fmt"

	"github.com/awslabs/operatorpkg/status"

	"github.com/aws/karpenter-provider-aws/pkg/providers/launchtemplate"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

type Readiness struct {
	launchTemplateProvider launchtemplate.Provider
}

func (n Readiness) Reconcile(ctx context.Context, nodeClass *v1.EC2NodeClass) (reconcile.Result, error) {
	// TODO: Drop runtime check once support for conversion is dropped, and make AMISelectorTerms required via CEL.
	if _, ok := nodeClass.Annotations[v1.AnnotationAMIFamilyCompatibility]; !ok && len(nodeClass.Spec.AMISelectorTerms) == 0 {
		nodeClass.StatusConditions().SetFalse(status.ConditionReady, "NodeClassNotReady", "Invalid AMI configuration")
		return reconcile.Result{}, fmt.Errorf("invalid configuration, AMISelectorTerms or 'karpenter.sh/v1beta1-amifamily' compatibility annotation must be specified")
	}
	// A NodeClass that uses AL2023 requires the cluster CIDR for launching nodes.
	// To allow Karpenter to be used for Non-EKS clusters, resolving the Cluster CIDR
	// will not be done at startup but instead in a reconcile loop.
	if nodeClass.AMIFamily() == v1.AMIFamilyAL2023 {
		if err := n.launchTemplateProvider.ResolveClusterCIDR(ctx); err != nil {
			nodeClass.StatusConditions().SetFalse(status.ConditionReady, "NodeClassNotReady", "Failed to detect the cluster CIDR")
			return reconcile.Result{}, fmt.Errorf("failed to detect the cluster CIDR, %w", err)
		}
	}
	return reconcile.Result{}, nil
}
