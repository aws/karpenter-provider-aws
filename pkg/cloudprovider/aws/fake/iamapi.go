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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
)

type IAMAPI struct {
	iamiface.IAMAPI
	GetInstanceProfileOutput *iam.GetInstanceProfileOutput
	WantErr                  error
}

func (a *IAMAPI) GetInstanceProfileWithContext(context.Context, *iam.GetInstanceProfileInput, ...request.Option) (*iam.GetInstanceProfileOutput, error) {
	if a.WantErr != nil {
		return nil, a.WantErr
	}
	if a.GetInstanceProfileOutput != nil {
		return a.GetInstanceProfileOutput, nil
	}
	return &iam.GetInstanceProfileOutput{
		InstanceProfile: &iam.InstanceProfile{
			InstanceProfileName: aws.String("KarpenterNodeInstanceProfile"),
			Roles:               []*iam.Role{{Arn: aws.String("test-role")}},
		},
	}, nil
}
