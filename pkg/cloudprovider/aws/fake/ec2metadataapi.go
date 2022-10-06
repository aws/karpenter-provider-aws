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

package fake

import (
	"context"

	"github.com/aws/aws-sdk-go/aws/ec2metadata"
)

type EC2MetadataAPI struct{}

func (e *EC2MetadataAPI) RegionWithContext(ctx context.Context) (string, error) {
	return "us-west-2", nil
}

func (e *EC2MetadataAPI) GetInstanceIdentityDocumentWithContext(context.Context) (ec2metadata.EC2InstanceIdentityDocument, error) {
	return ec2metadata.EC2InstanceIdentityDocument{
		AccountID: "000000000000",
	}, nil
}

func (e *EC2MetadataAPI) PartitionID() string {
	return "aws"
}
