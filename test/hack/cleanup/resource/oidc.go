package resource

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/samber/lo"
	"go.uber.org/multierr"
)

type OIDC struct {
	iamClient *iam.Client
}

func NewOIDC(iamClient *iam.Client) *OIDC {
	return &OIDC{iamClient: iamClient}
}

func (o *OIDC) Type() string {
	return "OpenIDConnectProvider"
}

func (o *OIDC) GetExpired(ctx context.Context, expirationTime time.Time) (names []string, err error) {
	out, err := o.iamClient.ListOpenIDConnectProviders(ctx, &iam.ListOpenIDConnectProvidersInput{})
	if err != nil {
		return names, err
	}

	errs := make([]error, len(out.OpenIDConnectProviderList))
	for i := range out.OpenIDConnectProviderList {
		oicd, err := o.iamClient.GetOpenIDConnectProvider(ctx, &iam.GetOpenIDConnectProviderInput{
			OpenIDConnectProviderArn: out.OpenIDConnectProviderList[i].Arn,
		})
		if err != nil {
			errs[i] = err
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

func (o *OIDC) Get(ctx context.Context, clusterName string) (names []string, err error) {
	return names, err
}

// Cleanup any old OIDC providers that were are remaining as part of testing
// We execute these in serial since we will most likely get rate limited if we try to delete these too aggressively
func (o *OIDC) Cleanup(ctx context.Context, arns []string) ([]string, error) {
	var errs error
	deleted := []string{}
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
