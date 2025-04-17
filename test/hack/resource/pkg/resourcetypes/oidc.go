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

package resourcetypes

import (
	"context"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	"golang.org/x/exp/slices"
)

type OIDC struct {
	iamClient *iam.Client
}

func NewOIDC(iamClient *iam.Client) *OIDC {
	return &OIDC{iamClient: iamClient}
}

func (o *OIDC) String() string {
	return "OpenIDConnectProvider"
}

func (o *OIDC) Global() bool {
	return true
}

func (o *OIDC) GetExpired(ctx context.Context, expirationTime time.Time, excludedClusters []string) (names []string, err error) {
	out, err := o.iamClient.ListOpenIDConnectProviders(ctx, &iam.ListOpenIDConnectProvidersInput{})
	if err != nil {
		return names, err
	}

	errs := make([]error, len(out.OpenIDConnectProviderList))
	for i := range out.OpenIDConnectProviderList {
		// Checking to make sure we are only list resources in the given region
		if !strings.Contains(lo.FromPtr(out.OpenIDConnectProviderList[i].Arn), lo.Must(config.LoadDefaultConfig(ctx)).Region) {
			continue
		}
		oicd, err := o.iamClient.GetOpenIDConnectProvider(ctx, &iam.GetOpenIDConnectProviderInput{
			OpenIDConnectProviderArn: out.OpenIDConnectProviderList[i].Arn,
		})
		if err != nil {
			errs[i] = err
			continue
		}

		clusterName, found := lo.Find(oicd.Tags, func(tag iamtypes.Tag) bool {
			return *tag.Key == k8sClusterTag
		})
		if found && slices.Contains(excludedClusters, lo.FromPtr(clusterName.Value)) {
			continue
		}

		for _, t := range oicd.Tags {
			if lo.FromPtr(t.Key) == githubRunURLTag && oicd.CreateDate.Before(expirationTime) {
				names = append(names, lo.FromPtr(out.OpenIDConnectProviderList[i].Arn))
			}
		}
	}

	return names, multierr.Combine(errs...)
}

func (o *OIDC) CountAll(ctx context.Context) (count int, err error) {
	out, err := o.iamClient.ListOpenIDConnectProviders(ctx, &iam.ListOpenIDConnectProvidersInput{})
	if err != nil {
		return count, err
	}

	return len(out.OpenIDConnectProviderList), nil
}

func (o *OIDC) Get(ctx context.Context, clusterName string) (names []string, err error) {
	return names, err
}

// Cleanup any old OIDC providers that were are remaining as part of testing
// We execute these in serial since we will most likely get rate limited if we try to delete these too aggressively
func (o *OIDC) Cleanup(ctx context.Context, arns []string) ([]string, error) {
	var deleted []string
	var errs error
	for i := range arns {
		_, err := o.iamClient.DeleteOpenIDConnectProvider(ctx, &iam.DeleteOpenIDConnectProviderInput{
			OpenIDConnectProviderArn: lo.ToPtr(arns[i]),
		})
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		deleted = append(deleted, arns[i])
	}
	return deleted, errs
}
