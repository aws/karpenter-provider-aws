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

package amifamily

import (
	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

type ResolvedNetworkInterface struct {
	NetworkCardIndex           int32
	DeviceIndex                int32
	InterfaceType              v1.InterfaceType
	SecondaryIPCount           *int32
	SecondaryIPPrefixCount     *int32
	SecondaryENISecurityGroups []v1.SecurityGroup
	SubnetID                   string
}

func ResolveNetworkInterfaces(nis []*v1.NetworkInterface) []*ResolvedNetworkInterface {
	if nis == nil {
		return nil
	}
	return lo.Map(nis, func(ni *v1.NetworkInterface, _ int) *ResolvedNetworkInterface {
		return &ResolvedNetworkInterface{
			NetworkCardIndex: ni.NetworkCardIndex,
			DeviceIndex:      ni.DeviceIndex,
			InterfaceType:    ni.InterfaceType,
		}
	})
}
