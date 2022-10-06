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

type MetadataProvider struct {
	imdsClient *ec2metadata.EC2Metadata
	stsClient  stsiface.STSAPI

	region   *string // cached region if already resolved
	regionMu sync.RWMutex

	accountID   *string // cached accountID if already resolved
	accountIDMu sync.RWMutex
}

func NewMetadataProvider(sess *session.Session) *MetadataProvider {
	return &MetadataProvider{
		imdsClient: ec2metadata.New(sess),
		stsClient:  sts.New(sess),
	}
}

// Region gets the current region from EC2 IMDS
func (i *MetadataProvider) Region(ctx context.Context) string {
	ret, err := cache.TryGetStringWithFallback(&i.regionMu, i.region,
		func() (string, error) {
			return i.imdsClient.RegionWithContext(ctx)
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
			doc, err := i.imdsClient.GetInstanceIdentityDocumentWithContext(ctx)
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
	return i.imdsClient.PartitionID
}
