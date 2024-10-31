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

package ssm

import (
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/samber/lo"
)

type Parameter struct {
	Name string
	// IsMutable indicates if the value associated with an SSM parameter is expected to change. An example of a mutable
	// parameter would be any of the "latest" or "recommended" AMI parameters which are updated each time a new AMI is
	// released. On the otherhand, we would consider a parameter parameter for a specific AMI version to be immutable.
	IsMutable bool
}

func (p *Parameter) GetParameterInput() *ssm.GetParameterInput {
	return &ssm.GetParameterInput{
		Name: lo.ToPtr(p.Name),
	}
}

func (p *Parameter) CacheKey() string {
	return p.Name
}

type CacheEntry struct {
	Parameter Parameter
	Value     string
}
