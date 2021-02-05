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

package packings

import (
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
)

type PackingMethod string

const (
	BinPacking PackingMethod = "binPacking"
)

// Factory returns a Packer to calculate the pod packing based of PackingMethod passed.
func Factory(ec2 ec2iface.EC2API, method PackingMethod) cloudprovider.Packer {
	switch method {
	case BinPacking:
		return &binPacker{ec2: ec2}
	}
	//TODO add more methods
	return &binPacker{ec2: ec2}
}
