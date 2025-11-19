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

// package reservedinstance provides types and methods for querying and managing
// EC2 Reserved Instances.
package reservedinstance

import (
	"context"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// ReservedInstance is a struct that defines the parameters for an EC2 Reserved Instance.
type ReservedInstance struct {
	ID               string
	InstanceType     ec2types.InstanceType
	InstanceCount    int32
	AvailabilityZone string
	State            ec2types.ReservedInstanceState
}

// Provider is an interface for getting reserved instance data.
type Provider interface {
	// GetReservedInstances returns all reserved instances for a given set of queries.
	GetReservedInstances(context.Context) ([]*ReservedInstance, error)
}