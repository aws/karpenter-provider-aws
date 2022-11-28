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

	"github.com/aws/amazon-ec2-spot-interrupter/pkg/itn"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/samber/lo"

	"github.com/aws/karpenter/pkg/controllers/interruption"
	"github.com/aws/karpenter/test/pkg/environment/common"
)

type Environment struct {
	*common.Environment
	Region string

	EC2API *ec2.EC2
	SSMAPI *ssm.SSM
	IAMAPI *iam.IAM

	SQSProvider     *interruption.SQSProvider
	InterruptionAPI *itn.ITN
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
		Region:          *session.Config.Region,
		Environment:     env,
		EC2API:          ec2.New(session),
		SSMAPI:          ssm.New(session),
		IAMAPI:          iam.New(session),
		InterruptionAPI: itn.New(lo.Must(config.LoadDefaultConfig(env.Context))),
		SQSProvider:     interruption.NewSQSProvider(sqs.New(session)),
	}
}

func (env *Environment) Stop() {
	env.Environment.Stop()
}
