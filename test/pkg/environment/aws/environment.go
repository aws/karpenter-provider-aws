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
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/fis"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/samber/lo"
	"k8s.io/utils/env"

	. "github.com/onsi/ginkgo/v2" //nolint:revive,stylecheck

	"github.com/aws/karpenter/pkg/controllers/interruption"
	"github.com/aws/karpenter/test/pkg/environment/common"
)

const WindowsDefaultImage = "mcr.microsoft.com/oss/kubernetes/pause:3.9"

type Environment struct {
	*common.Environment
	Region string

	STSAPI        *sts.STS
	EC2API        *ec2.EC2
	SSMAPI        *ssm.SSM
	IAMAPI        *iam.IAM
	FISAPI        *fis.FIS
	EKSAPI        *eks.EKS
	CloudwatchAPI cloudwatchiface.CloudWatchAPI

	SQSProvider *interruption.SQSProvider
}

func NewEnvironment(t *testing.T) *Environment {
	env := common.NewEnvironment(t)
	session := session.Must(session.NewSessionWithOptions(
		session.Options{
			Config: *request.WithRetryer(
				&aws.Config{STSRegionalEndpoint: endpoints.RegionalSTSEndpoint},
				client.DefaultRetryer{NumMaxRetries: 10},
			),
			SharedConfigState: session.SharedConfigEnable,
		},
	))

	return &Environment{
		Region:      *session.Config.Region,
		Environment: env,

		STSAPI:        sts.New(session),
		EC2API:        ec2.New(session),
		SSMAPI:        ssm.New(session),
		IAMAPI:        iam.New(session),
		FISAPI:        fis.New(session),
		EKSAPI:        eks.New(session),
		CloudwatchAPI: GetCloudWatchAPI(session),
		SQSProvider:   interruption.NewSQSProvider(sqs.New(session)),
	}
}

func GetCloudWatchAPI(session *session.Session) cloudwatchiface.CloudWatchAPI {
	if lo.Must(env.GetBool("ENABLE_CLOUDWATCH", false)) {
		By("enabling cloudwatch metrics firing for this suite")
		return cloudwatch.New(session)
	}
	return &NoOpCloudwatchAPI{}
}
