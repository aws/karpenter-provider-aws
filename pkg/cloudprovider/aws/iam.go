package aws

import (
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
)

type IAMProvider struct {
	client iamiface.IAMAPI
}

func NewIAMProvider(api iamiface.IAMAPI) *IAMProvider {
	return &IAMProvider{
		client: api,
	}
}
