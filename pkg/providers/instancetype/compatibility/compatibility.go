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
	"strings"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

type NodeClass interface {
	AMIFamily() string
}

type CompatibleCheck interface {
	compatibleCheck(info ec2types.InstanceTypeInfo) bool
}

func IsCompatibleWithNodeClass(info ec2types.InstanceTypeInfo, nodeClass NodeClass) bool {
	for _, check := range []CompatibleCheck{
		amiFamilyCompatibility(nodeClass.AMIFamily()),
	} {
		if !check.compatibleCheck(info) {
			return false
		}
	}
	return true
}

type amiFamilyCheck struct {
	amiFamily string
}

func amiFamilyCompatibility(amiFamily string) CompatibleCheck {
	return &amiFamilyCheck{
		amiFamily: amiFamily,
	}
}

func (c amiFamilyCheck) compatibleCheck(info ec2types.InstanceTypeInfo) bool {
	// a1 instance types are not supported with al2023s (https://docs.aws.amazon.com/linux/al2023/ug/system-requirements.html)
	if c.amiFamily == v1.AMIFamilyAL2023 && strings.HasPrefix(string(info.InstanceType), "a1.") {
		return false
	}
	return true
}
