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

package aws

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/awslabs/karpenter/pkg/cloudprovider/aws/fake"
)

var (
	launchTemplate = "test-templateID"
	version        = "10"
	az             = "us-east-2b"
	subnetID       = "subnet-12345dead789"
)

var FleetCreateSampleOutput = `
{
	FleetId: "fleet-44f501fd-d40a-ecd4-acba-032a99d6b167",
	Instances: [{
		InstanceIds: ["i-0dfdacec11d0d5bb3"],
		InstanceType: "t2.medium",
		LaunchTemplateAndOverrides: {
			LaunchTemplateSpecification: {
			LaunchTemplateId: "lt-02f427483e1be00f5",
			Version: "6"
			},
			Overrides: {
			AvailabilityZone: "us-east-2b",
			SubnetId: "subnet-0f5bae1584d67d456"
			}
		},
		Lifecycle: "on-demand"
	}]
}
`

func Test_instanceConfig_Create(t *testing.T) {
	type fields struct {
		ec2Iface       ec2iface.EC2API
		templateConfig *ec2.FleetLaunchTemplateConfigRequest
		capacitySpec   *ec2.TargetCapacitySpecificationRequest
		instanceID     string
	}
	type args struct {
		ctx context.Context
	}

	ec2Iface := fake.EC2API{FleetOutput: &ec2.CreateFleetOutput{}, WantErr: nil}
	cfg := NewFleetRequest(launchTemplate, version, ec2Iface)

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "First basic test",
			fields: fields{
				ec2Iface:       ec2Iface,
				templateConfig: cfg.templateConfig,
				capacitySpec:   cfg.capacitySpec,
			},
			args: args{context.Background()},
		},
	}
	cfg.SetAvailabilityZone(az)
	cfg.SetInstanceType(ec2.DefaultTargetCapacityTypeOnDemand)
	cfg.SetOnDemandCapacity(1, 1)
	cfg.SetSubnet(subnetID)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &instanceConfig{
				ec2Iface:       tt.fields.ec2Iface,
				templateConfig: tt.fields.templateConfig,
				capacitySpec:   tt.fields.capacitySpec,
				instanceID:     tt.fields.instanceID,
			}
			if err := cfg.Create(tt.args.ctx); (err != nil) != tt.wantErr {
				t.Errorf("instanceConfig.Create() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
