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

	"github.com/aws/aws-sdk-go-v2/service/arczonalshift"

	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
)

type ARCZonalShiftAPI struct {
	sdk.ARCZonalShiftAPI
	GetManagedResourceBehavior MockedFunction[arczonalshift.GetManagedResourceInput, arczonalshift.GetManagedResourceOutput]
}

func NewARCZonalShiftAPI() *ARCZonalShiftAPI {
	return &ARCZonalShiftAPI{}
}

func (a *ARCZonalShiftAPI) GetManagedResource(ctx context.Context, input *arczonalshift.GetManagedResourceInput, _ ...func(*arczonalshift.Options)) (*arczonalshift.GetManagedResourceOutput, error) {
	return a.GetManagedResourceBehavior.Invoke(input, func(*arczonalshift.GetManagedResourceInput) (*arczonalshift.GetManagedResourceOutput, error) {
		return &arczonalshift.GetManagedResourceOutput{}, nil
	})
}

func (a *ARCZonalShiftAPI) Reset() {
	a.GetManagedResourceBehavior.Reset()
}
