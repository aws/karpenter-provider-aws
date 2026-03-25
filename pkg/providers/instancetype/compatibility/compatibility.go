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

package compatibility

import (
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

type NodeClass interface {
	NetworkInterfaces() []*v1.NetworkInterface
}

type CompatibleCheck interface {
	compatibleCheck(info ec2types.InstanceTypeInfo) bool
}

func IsCompatibleWithNodeClass(info ec2types.InstanceTypeInfo, nodeClass NodeClass) bool {
	for _, check := range []CompatibleCheck{
		networkInterfaceCompatibility(nodeClass.NetworkInterfaces()),
	} {
		if !check.compatibleCheck(info) {
			return false
		}
	}
	return true
}

type networkInterfaceCheck struct {
	networkInterfaces []*v1.NetworkInterface
}

func networkInterfaceCompatibility(networkInterfaces []*v1.NetworkInterface) CompatibleCheck {
	return &networkInterfaceCheck{
		networkInterfaces: networkInterfaces,
	}
}

//nolint:gocyclo
func (c networkInterfaceCheck) compatibleCheck(info ec2types.InstanceTypeInfo) bool {
	if c.networkInterfaces == nil {
		return true
	}
	// Not all instance types are compatible with network interfaces defined on the Node Class.
	// We check for 4 cases here. Note that that this is not intended to catch every configuration.
	if info.NetworkInfo == nil || len(info.NetworkInfo.NetworkCards) == 0 {
		return false
	}
	// (1) the instance type supports ENA interfaces
	if info.NetworkInfo.EnaSupport == ec2types.EnaSupportUnsupported {
		return false
	}
	for _, networkInterface := range c.networkInterfaces {
		// (2) the configured number of network cards is greater than what the instance type offers
		nci := networkInterface.NetworkCardIndex
		if len(info.NetworkInfo.NetworkCards) <= int(nci) {
			return false
		}
		// (3) the configured number of device indices for a network card is greater than what the instance offers
		if lo.FromPtr(info.NetworkInfo.NetworkCards[nci].MaximumNetworkInterfaces) <= networkInterface.DeviceIndex {
			return false
		}
	}
	// (4) the configured number of EFA-only interfaces is greater than what the instance type offers
	numEfas := lo.CountBy(c.networkInterfaces, func(nic *v1.NetworkInterface) bool {
		return nic.InterfaceType == v1.InterfaceTypeEFAOnly
	})
	if numEfas > 0 {
		if info.NetworkInfo.EfaInfo == nil || info.NetworkInfo.EfaInfo.MaximumEfaInterfaces == nil {
			return false
		}
		if numEfas > int(lo.FromPtr(info.NetworkInfo.EfaInfo.MaximumEfaInterfaces)) {
			return false
		}
	}
	return true
}
