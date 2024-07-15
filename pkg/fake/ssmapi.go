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
	"log"
	"regexp"
	"strings"

	"github.com/Pallinder/go-randomdata"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/samber/lo"

	"github.com/aws/karpenter-provider-aws/pkg/providers/version"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
)

type SSMAPI struct {
	ssmiface.SSMAPI
	Parameters                map[string]string
	GetParameterOutput        *ssm.GetParameterOutput
	GetParametersByPathOutput *ssm.GetParametersByPathOutput
	WantErr                   error
}

func NewSSMAPI() *SSMAPI {
	return &SSMAPI{}
}

func (a SSMAPI) GetParameterWithContext(_ context.Context, input *ssm.GetParameterInput, _ ...request.Option) (*ssm.GetParameterOutput, error) {
	if a.WantErr != nil {
		return nil, a.WantErr
	}
	if len(a.Parameters) > 0 {
		if amiID, ok := a.Parameters[*input.Name]; ok {
			return &ssm.GetParameterOutput{
				Parameter: &ssm.Parameter{Value: aws.String(amiID)},
			}, nil
		}
		return nil, awserr.New(ssm.ErrCodeParameterNotFound, fmt.Sprintf("%s couldn't be found", *input.Name), nil)
	}
	hc, _ := hashstructure.Hash(input.Name, hashstructure.FormatV2, nil)
	if a.GetParameterOutput != nil {
		return a.GetParameterOutput, nil
	}

	return &ssm.GetParameterOutput{
		Parameter: &ssm.Parameter{Value: aws.String(fmt.Sprintf("test-ami-id-%x", hc))},
	}, nil
}

func (a SSMAPI) GetParametersByPathPagesWithContext(_ context.Context, input *ssm.GetParametersByPathInput, f func(*ssm.GetParametersByPathOutput, bool) bool, _ ...request.Option) error {
	if !lo.FromPtr(input.Recursive) {
		log.Fatalf("fake SSM API currently only supports GetParametersByPathPagesWithContext when recursive is true")
	}
	if a.WantErr != nil {
		return a.WantErr
	}
	if a.GetParametersByPathOutput != nil {
		f(a.GetParametersByPathOutput, true)
		return nil
	}
	if len(a.Parameters) != 0 {
		f(&ssm.GetParametersByPathOutput{
			Parameters: lo.FilterMap(lo.Entries(a.Parameters), func(p lo.Entry[string, string], _ int) (*ssm.Parameter, bool) {
				// The parameter does not start with the path
				if !strings.HasPrefix(p.Key, lo.FromPtr(input.Path)) {
					return nil, false
				}
				// The parameter starts with the input path, but the last segment of the input path is only a subset of the matching segment of the parameters path.
				// Ex: "/aws/service/eks-optimized-ami/amazon-linux-2" is a prefix for "/aws/service/eks-optimized-ami/amazon-linux-2-gpu/..." but we shouldn't match
				if strings.TrimPrefix(p.Key, lo.FromPtr(input.Path))[0] != '/' {
					return nil, false
				}
				return &ssm.Parameter{
					Name:  lo.ToPtr(p.Key),
					Value: lo.ToPtr(p.Value),
				}, true
			}),
		}, true)
		return nil
	}
	if params := getDefaultParametersForPath(lo.FromPtr(input.Path)); params != nil {
		f(&ssm.GetParametersByPathOutput{Parameters: params}, true)
		return nil
	}
	return fmt.Errorf("path %q does not exist", lo.FromPtr(input.Path))
}

func getDefaultParametersForPath(path string) []*ssm.Parameter {
	suffixes := map[string][]string{
		`^\/aws\/service\/eks/optimized-ami\/.*\/amazon-linux-2$`:       []string{"recommended/image_id"},
		`^\/aws\/service\/eks/optimized-ami\/.*\/amazon-linux-2-arm64$`: []string{"recommended/image_id"},
		`^\/aws\/service\/eks/optimized-ami\/.*\/amazon-linux-2-gpu$`:   []string{"recommended/image_id"},
		`^\/aws\/service\/eks/optimized-ami\/.*\/amazon-linux-2023$`: []string{
			"x86_64/standard/recommended/image_id",
			"arm64/standard/recommended/image_id",
			"x86_64/nvidia/recommended/image_id",
			"arm64/nvidia/recommended/image_id",
			"x86_64/neuron/recommended/image_id",
			"arm64/neuron/recommended/image_id",
		},
		`\/aws\/service\/bottlerocket\/aws-k8s-.*`: []string{
			"x86_64/latest/image_id",
			"arm64/latest/image_id",
		},
		`\/aws\/service\/ami-windows-latest`: lo.FlatMap(version.SupportedK8sVersions(), func(version string, _ int) []string {
			return []string{
				fmt.Sprintf("Windows_Server-2019-English-Core-EKS_Optimized-%s/image_id", version),
				fmt.Sprintf("Windows_Server-2022-English-Core-EKS_Optimized-%s/image_id", version),
			}
		}),
	}
	for matchStr, suffixes := range suffixes {
		if !regexp.MustCompile(matchStr).MatchString(path) {
			continue
		}
		return lo.Map(suffixes, func(suffix string, _ int) *ssm.Parameter {
			return &ssm.Parameter{
				Name:  lo.ToPtr(fmt.Sprintf("%s/%s", path, suffix)),
				Value: lo.ToPtr(fmt.Sprintf("ami-%s", randomdata.Alphanumeric(16))),
			}
		})
	}
	return nil
}

func (a *SSMAPI) Reset() {
	a.GetParameterOutput = nil
	a.Parameters = nil
	a.WantErr = nil
}
