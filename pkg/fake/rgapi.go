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
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/resourcegroups"
	"github.com/aws/aws-sdk-go/service/resourcegroups/resourcegroupsiface"
)

type ResourceGroupsAPI struct {
	resourcegroupsiface.ResourceGroupsAPI
	ResourceGroupsBehaviour
}
type ResourceGroupsBehaviour struct {
	NextError        AtomicError
	ListGroupsOutput AtomicPtr[resourcegroups.ListGroupsOutput]
}

func (r *ResourceGroupsAPI) Reset() {
	r.NextError.Reset()
    r.ListGroupsOutput.Reset()
}

func (r *ResourceGroupsAPI) ListGroups(_ aws.Context, _ *resourcegroups.ListGroupsInput, fn func(*resourcegroups.ListGroupsOutput, bool) bool, _ ...request.Option) error {
	if !r.NextError.IsNil() {
		return r.NextError.Get()
	}
	if !r.ListGroupsOutput.IsNil() {
		fn(r.ListGroupsOutput.Clone(), false)
		return nil
	}
	// fail if the test doesn't provide specific data which causes our pricing provider to use its static price list
	return errors.New("no resource groups provided")
}
