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
	"github.com/aws/aws-sdk-go/service/resourcegroups"
	"github.com/aws/aws-sdk-go/service/resourcegroups/resourcegroupsiface"
)

var (
	TestLicenseArn  = "arn:aws:license-manager:us-west-2:11111111111:license-configuration:lic-94ba36399bd98eaad808b0ffb1d1604b"
	TestLicenseName = "test-license"
)

type ResourceGroupsAPI struct {
	resourcegroupsiface.ResourceGroupsAPI
	ResourceGroupsBehaviour
}
type ResourceGroupsBehaviour struct {
	NextError          AtomicError
	ListGroupsBehavior MockedFunction[resourcegroups.ListGroupsInput, resourcegroups.ListGroupsOutput]
}

func (r *ResourceGroupsAPI) Reset() {
	r.NextError.Reset()
    r.ListGroupsBehavior.Reset()
}

func (r *ResourceGroupsAPI) ListGroupsWithContext(_ aws.Context, input *resourcegroups.ListGroupsInput, _ ...request.Option) (*resourcegroups.ListGroupsOutput, error) {
	if !r.NextError.IsNil() {
		return nil, r.NextError.Get()
	}
	return r.ListGroupsBehavior.Invoke(input, func(input *resourcegroups.ListGroupsInput) (*resourcegroups.ListGroupsOutput, error) {
		return &resourcegroups.ListGroupsOutput{
			GroupIdentifiers: []*resourcegroups.GroupIdentifier{
				{GroupArn: aws.String(TestLicenseArn), GroupName: aws.String(TestLicenseName)},
			},
		}, nil
	})
}

func (r *ResourceGroupsAPI) ListGroupsPagesWithContext(ctx context.Context, input *resourcegroups.ListGroupsInput, fn func(*resourcegroups.ListGroupsOutput, bool) bool, opts ...request.Option) error {
	output, err := r.ListGroupsWithContext(ctx, input, opts...)
	if err != nil {
		return err
	}
	fn(output, false)
	return nil
}
