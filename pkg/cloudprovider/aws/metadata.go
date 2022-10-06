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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/utils/atomic"
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
	sess              *session.Session

	region    atomic.CachedVal[string] // cached region if already resolved
	accountID atomic.CachedVal[string] // cached accountID if already resolved
}

func NewMetadataProvider(sess *session.Session, ec2metadataapi EC2MetadataInterface, stsapi stsiface.STSAPI) *MetadataProvider {
	m := &MetadataProvider{
		ec2MetadataClient: ec2metadataapi,
		stsClient:         stsapi,
		sess:              sess,
	}
	m.region.Resolve = func(ctx context.Context) (string, error) {
		if m.sess != nil && m.sess.Config != nil && m.sess.Config.Region != nil && *m.sess.Config.Region != "" {
			return *m.sess.Config.Region, nil
		}
		logging.FromContext(ctx).Debug("AWS region not configured, asking EC2 Instance Metadata Service")
		return m.ec2MetadataClient.RegionWithContext(ctx)
	}
	m.accountID.Resolve = func(ctx context.Context) (string, error) {
		doc, err := m.ec2MetadataClient.GetInstanceIdentityDocumentWithContext(ctx)
		if err != nil {
			// Resolve to using the STS provider if IMDS fails
			result, err := m.stsClient.GetCallerIdentity(&sts.GetCallerIdentityInput{})
			if err != nil {
				return "", err
			}
			return aws.StringValue(result.Account), nil
		}
		return doc.AccountID, nil
	}
	return m
}

// MustEnsureRegion resolves the region set in the session config if not already set
func (m *MetadataProvider) MustEnsureRegion(ctx context.Context, sess *session.Session) {
	ret, err := m.Region(ctx)
	if err != nil {
		panic(fmt.Sprintf("Failed to ensure session region, %v", err))
	}
	*sess.Config.Region = ret
}

// Region gets the current region from EC2 IMDS
func (m *MetadataProvider) Region(ctx context.Context) (string, error) {
	return m.region.TryGet(ctx)
}

// AccountID gets the AWS Account ID from EC2 IMDS, then STS if it can't be resolved at IMDS
func (m *MetadataProvider) AccountID(ctx context.Context) (string, error) {
	return m.accountID.TryGet(ctx)
}

func (m *MetadataProvider) Partition() string {
	return m.ec2MetadataClient.PartitionID()
}
