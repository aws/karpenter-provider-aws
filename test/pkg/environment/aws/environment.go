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
	"fmt"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/fis"
	"github.com/aws/aws-sdk-go/service/iam"
	servicesqs "github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/timestreamwrite"
	"github.com/aws/aws-sdk-go/service/timestreamwrite/timestreamwriteiface"
	. "github.com/onsi/ginkgo/v2"
	"github.com/samber/lo"
	"k8s.io/utils/env"

	"github.com/aws/karpenter/pkg/apis/v1beta1"
	"github.com/aws/karpenter/pkg/providers/sqs"
	"github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/test/pkg/environment/common"
)

const WindowsDefaultImage = "mcr.microsoft.com/oss/kubernetes/pause:3.9"

// ExcludedInstanceFamilies denotes instance families that have issues during resource registration due to compatibility
// issues with versions of the VPR Resource Controller
var ExcludedInstanceFamilies = []string{"m7a", "r7a", "c7a", "r7i"}

type Environment struct {
	*common.Environment
	Region string

	STSAPI        *sts.STS
	EC2API        *ec2.EC2
	SSMAPI        *ssm.SSM
	IAMAPI        *iam.IAM
	FISAPI        *fis.FIS
	EKSAPI        *eks.EKS
	TimeStreamAPI timestreamwriteiface.TimestreamWriteAPI

	SQSProvider *sqs.Provider

	ClusterName       string
	ClusterEndpoint   string
	InterruptionQueue string
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

	awsEnv := &Environment{
		Region:      *session.Config.Region,
		Environment: env,

		STSAPI:        sts.New(session),
		EC2API:        ec2.New(session),
		SSMAPI:        ssm.New(session),
		IAMAPI:        iam.New(session),
		FISAPI:        fis.New(session),
		EKSAPI:        eks.New(session),
		TimeStreamAPI: GetTimeStreamAPI(session),

		ClusterName:     lo.Must(os.LookupEnv("CLUSTER_NAME")),
		ClusterEndpoint: lo.Must(os.LookupEnv("CLUSTER_ENDPOINT")),
	}
	// Initialize the provider only if the INTERRUPTION_QUEUE environment variable is defined
	if v, ok := os.LookupEnv("INTERRUPTION_QUEUE"); ok {
		awsEnv.SQSProvider = lo.Must(sqs.NewProvider(env.Context, servicesqs.New(session), v))
	}
	return awsEnv
}

func GetTimeStreamAPI(session *session.Session) timestreamwriteiface.TimestreamWriteAPI {
	if lo.Must(env.GetBool("ENABLE_METRICS", false)) {
		By("enabling metrics firing for this suite")
		return timestreamwrite.New(session, &aws.Config{Region: aws.String(env.GetString("METRICS_REGION", metricsDefaultRegion))})
	}
	return &NoOpTimeStreamAPI{}
}

func (env *Environment) DefaultEC2NodeClass() *v1beta1.EC2NodeClass {
	nodeClass := test.EC2NodeClass()
	nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyAL2
	nodeClass.Spec.Tags = map[string]string{
		"testing/cluster": env.ClusterName,
	}
	nodeClass.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
		{
			Tags: map[string]string{"karpenter.sh/discovery": env.ClusterName},
		},
	}
	nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
		{
			Tags: map[string]string{"karpenter.sh/discovery": env.ClusterName},
		},
	}
	nodeClass.Spec.Role = fmt.Sprintf("KarpenterNodeRole-%s", env.ClusterName)
	return nodeClass
}
