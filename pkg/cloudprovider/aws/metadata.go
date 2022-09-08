package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
)

type MetadataProvider struct {
	imdsClient *ec2metadata.EC2Metadata
	stsClient  stsiface.STSAPI
}

func NewMetadataProvider(sess *session.Session) *MetadataProvider {
	return &MetadataProvider{
		imdsClient: ec2metadata.New(sess),
		stsClient:  sts.New(sess),
	}
}

// Region gets the current region from EC2 IMDS
func (i *MetadataProvider) Region(ctx context.Context) string {
	region, err := i.imdsClient.RegionWithContext(ctx)
	if err != nil {
		panic(fmt.Sprintf("Failed to call the metadata server's region API, %s", err))
	}
	return region
}

func (i *MetadataProvider) AccountID(ctx context.Context) string {
	doc, err := i.imdsClient.GetInstanceIdentityDocumentWithContext(ctx)
	if err != nil {
		// Fallback to using the STS provider if IMDS fails
		result, err := i.stsClient.GetCallerIdentity(&sts.GetCallerIdentityInput{})
		if err != nil {
			panic(fmt.Sprintf("Failed to get account ID from IMDS or STS, %s", err))
		}
		return aws.StringValue(result.Account)
	}
	return doc.AccountID
}
