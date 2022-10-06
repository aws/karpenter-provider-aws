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
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/utils/cache"
)

type MetadataProvider struct {
	imdsClient *ec2metadata.EC2Metadata
	stsClient  stsiface.STSAPI
	sess       *session.Session

	region   *string // cached region if already resolved
	regionMu sync.RWMutex

	accountID   *string // cached accountID if already resolved
	accountIDMu sync.RWMutex
}

func NewMetadataProvider(sess *session.Session) *MetadataProvider {
	return &MetadataProvider{
		imdsClient: ec2metadata.New(sess),
		stsClient:  sts.New(sess),
		sess:       sess,
	}
}

// EnsureSessionRegion resolves the region set in the session config if not already set
func (m *MetadataProvider) EnsureSessionRegion(ctx context.Context, sess *session.Session) {
	*sess.Config.Region = m.Region(ctx)
}

// Region gets the current region from EC2 IMDS
func (m *MetadataProvider) Region(ctx context.Context) string {
	ret, err := cache.TryGetStringWithFallback(&m.regionMu, m.region,
		func() (string, error) {
			if m.sess != nil && m.sess.Config != nil && m.sess.Config.Region != nil && *m.sess.Config.Region != "" {
				return *m.sess.Config.Region, nil
			}
			logging.FromContext(ctx).Debug("AWS region not configured, asking EC2 Instance Metadata Service")
			return m.imdsClient.RegionWithContext(ctx)
		})
	if err != nil {
		panic(fmt.Sprintf("Failed to call the metadata server's region API, %s", err))
	}
	return ret
}

// AccountID gets the AWS Account ID from EC2 IMDS, then STS if it can't be resolved at IMDS
func (m *MetadataProvider) AccountID(ctx context.Context) string {
	ret, err := cache.TryGetStringWithFallback(&m.accountIDMu, m.accountID,
		func() (string, error) {
			doc, err := m.imdsClient.GetInstanceIdentityDocumentWithContext(ctx)
			if err != nil {
				// Fallback to using the STS provider if IMDS fails
				result, err := m.stsClient.GetCallerIdentity(&sts.GetCallerIdentityInput{})
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

func (m *MetadataProvider) Partition() string {
	return m.imdsClient.PartitionID
}
