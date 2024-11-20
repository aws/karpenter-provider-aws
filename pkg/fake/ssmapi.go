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
	"fmt"

	"github.com/Pallinder/go-randomdata"
	"github.com/samber/lo"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
)

type SSMAPI struct {
	sdk.SSMAPI
	Parameters         map[string]string
	GetParameterOutput *ssm.GetParameterOutput
	WantErr            error

	defaultParameters map[string]string
}

func NewSSMAPI() *SSMAPI {
	return &SSMAPI{
		defaultParameters: map[string]string{},
	}
}

func (a SSMAPI) GetParameter(_ context.Context, input *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	parameter := lo.FromPtr(input.Name)
	if a.WantErr != nil {
		return &ssm.GetParameterOutput{}, a.WantErr
	}
	if a.GetParameterOutput != nil {
		return a.GetParameterOutput, nil
	}
	if len(a.Parameters) != 0 {
		value, ok := a.Parameters[parameter]
		if !ok {
			return &ssm.GetParameterOutput{}, fmt.Errorf("parameter %q not found", lo.FromPtr(input.Name))
		}
		return &ssm.GetParameterOutput{
			Parameter: &ssmtypes.Parameter{
				Name:  lo.ToPtr(parameter),
				Value: lo.ToPtr(value),
			},
		}, nil
	}

	// Cache default parameters that was successive calls for the same parameter return the same result
	value, ok := a.defaultParameters[parameter]
	if !ok {
		value = fmt.Sprintf("ami-%s", randomdata.Alphanumeric(16))
		a.defaultParameters[parameter] = value
	}
	return &ssm.GetParameterOutput{
		Parameter: &ssmtypes.Parameter{
			Name:  lo.ToPtr(parameter),
			Value: lo.ToPtr(value),
		},
	}, nil
}

func (a *SSMAPI) Reset() {
	a.Parameters = nil
	a.GetParameterOutput = nil
	a.WantErr = nil
	a.defaultParameters = map[string]string{}
}
