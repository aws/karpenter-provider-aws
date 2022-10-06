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

package aws

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"

	"github.com/aws/karpenter/pkg/utils/cache"
)

type EC2MetadataInterface interface {
	RegionWithContext(context.Context) (string, error)
	GetInstanceIdentityDocumentWithContext(context.Context) (ec2metadata.EC2InstanceIdentityDocument, error)
	PartitionID() string
}

type EC2MetadataClient struct {
	*ec2metadata.EC2Metadata
}

func NewEC2MetadataClient(sess *session.Session) *EC2MetadataClient {
	return &EC2MetadataClient{
		EC2Metadata: ec2metadata.New(sess),
	}
}

func (e *EC2MetadataClient) PartitionID() string {
	return e.EC2Metadata.PartitionID
}

type MetadataProvider struct {
	ec2MetadataClient EC2MetadataInterface
	stsClient         stsiface.STSAPI

	region   *string // cached region if already resolved
	regionMu sync.RWMutex

	accountID   *string // cached accountID if already resolved
	accountIDMu sync.RWMutex
}

func NewMetadataProvider(ec2metadataapi EC2MetadataInterface, stsapi stsiface.STSAPI) *MetadataProvider {
	return &MetadataProvider{
		ec2MetadataClient: ec2metadataapi,
		stsClient:         stsapi,
	}
}

// Region gets the current region from EC2 IMDS
func (i *MetadataProvider) Region(ctx context.Context) string {
	ret, err := cache.TryGetStringWithFallback(&i.regionMu, i.region,
		func() (string, error) {
			return i.ec2MetadataClient.RegionWithContext(ctx)
		})
	if err != nil {
		panic(fmt.Sprintf("Failed to call the metadata server's region API, %s", err))
	}
	return ret
}

// AccountID gets the AWS Account ID from EC2 IMDS, then STS if it can't be resolved at IMDS
func (i *MetadataProvider) AccountID(ctx context.Context) string {
	ret, err := cache.TryGetStringWithFallback(&i.accountIDMu, i.accountID,
		func() (string, error) {
			doc, err := i.ec2MetadataClient.GetInstanceIdentityDocumentWithContext(ctx)
			if err != nil {
				// Fallback to using the STS provider if IMDS fails
				result, err := i.stsClient.GetCallerIdentity(&sts.GetCallerIdentityInput{})
				if err != nil {
					return "", err
				}
				return aws.StringValue(result.Account), nil
			}
			return doc.AccountID, nil
		},
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to get account ID from IMDS or STS, %s", err))
	}
	return ret
}

func (i *MetadataProvider) Partition() string {
	return i.ec2MetadataClient.PartitionID()
}
