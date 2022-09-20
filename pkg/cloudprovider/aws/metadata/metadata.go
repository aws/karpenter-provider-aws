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

package metadata

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
)

type Provider struct {
	imdsClient *ec2metadata.EC2Metadata
	stsClient  stsiface.STSAPI
}

func NewMetadataProvider(sess *session.Session) *Provider {
	return &Provider{
		imdsClient: ec2metadata.New(sess),
		stsClient:  sts.New(sess),
	}
}

// Region gets the current region from EC2 IMDS
func (i *Provider) Region(ctx context.Context) string {
	region, err := i.imdsClient.RegionWithContext(ctx)
	if err != nil {
		panic(fmt.Sprintf("Failed to call the metadata server's region API, %s", err))
	}
	return region
}

func (i *Provider) AccountID(ctx context.Context) string {
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
