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

package nodeclass

import (
	"context"
	"fmt"
	"sort"
	"time"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/subnet"
)

type Subnet struct {
	subnetProvider subnet.Provider
}

func NewSubnetReconciler(subnetProvider subnet.Provider) *Subnet {
	return &Subnet{
		subnetProvider: subnetProvider,
	}
}

func (s *Subnet) Reconcile(ctx context.Context, nodeClass *v1.EC2NodeClass) (reconcile.Result, error) {
	subnets, err := s.subnetProvider.List(ctx, nodeClass)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("getting subnets, %w", err)
	}
	if len(subnets) == 0 {
		nodeClass.Status.Subnets = nil
		nodeClass.StatusConditions().SetFalse(v1.ConditionTypeSubnetsReady, "SubnetsNotFound", "SubnetSelector did not match any Subnets")
		// If users have omitted the necessary tags from their Subnets and later add them, we need to reprocess the information.
		// Returning 'ok' in this case means that the nodeclass will remain in an unready state until the component is restarted.
		return reconcile.Result{RequeueAfter: time.Minute}, nil
	}
	sort.Slice(subnets, func(i, j int) bool {
		if int(*subnets[i].AvailableIpAddressCount) != int(*subnets[j].AvailableIpAddressCount) {
			return int(*subnets[i].AvailableIpAddressCount) > int(*subnets[j].AvailableIpAddressCount)
		}
		return *subnets[i].SubnetId < *subnets[j].SubnetId
	})
	nodeClass.Status.Subnets = lo.Map(subnets, func(ec2subnet ec2types.Subnet, _ int) v1.Subnet {
		return v1.Subnet{
			ID:     *ec2subnet.SubnetId,
			Zone:   *ec2subnet.AvailabilityZone,
			ZoneID: *ec2subnet.AvailabilityZoneId,
			VpcID:  *ec2subnet.VpcId,
		}
	})
	nodeClass.StatusConditions().SetTrue(v1.ConditionTypeSubnetsReady)
	return reconcile.Result{RequeueAfter: time.Minute}, nil
}
