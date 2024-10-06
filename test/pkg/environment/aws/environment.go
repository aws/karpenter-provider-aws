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

	coretest "sigs.k8s.io/karpenter/pkg/test"

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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/env"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/sqs"
	"github.com/aws/karpenter-provider-aws/pkg/test"
	"github.com/aws/karpenter-provider-aws/test/pkg/environment/common"
)

func init() {
	karpv1.NormalizedLabels = lo.Assign(karpv1.NormalizedLabels, map[string]string{"topology.ebs.csi.aws.com/zone": corev1.LabelTopologyZone})
}

var WindowsDefaultImage = "mcr.microsoft.com/oss/kubernetes/pause:3.9"

var EphemeralInitContainerImage = "alpine"

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

	SQSProvider sqs.Provider

	ClusterName       string
	ClusterEndpoint   string
	InterruptionQueue string
	PrivateCluster    bool
	ZoneInfo          []ZoneInfo
}

type ZoneInfo struct {
	Zone     string
	ZoneID   string
	ZoneType string
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

	if _, awsEnv.PrivateCluster = os.LookupEnv("PRIVATE_CLUSTER"); awsEnv.PrivateCluster {
		WindowsDefaultImage = fmt.Sprintf("857221689048.dkr.ecr.%s.amazonaws.com/k8s/pause:3.6", awsEnv.Region)
		EphemeralInitContainerImage = fmt.Sprintf("857221689048.dkr.ecr.%s.amazonaws.com/ecr-public/docker/library/alpine:latest", awsEnv.Region)
		coretest.DefaultImage = fmt.Sprintf("857221689048.dkr.ecr.%s.amazonaws.com/ecr-public/eks-distro/kubernetes/pause:3.2", awsEnv.Region)
	}
	// Initialize the provider only if the INTERRUPTION_QUEUE environment variable is defined
	if v, ok := os.LookupEnv("INTERRUPTION_QUEUE"); ok {
		sqsapi := servicesqs.New(session)
		out := lo.Must(sqsapi.GetQueueUrlWithContext(env.Context, &servicesqs.GetQueueUrlInput{QueueName: aws.String(v)}))
		awsEnv.SQSProvider = lo.Must(sqs.NewDefaultProvider(sqsapi, lo.FromPtr(out.QueueUrl)))
	}
	// Populate ZoneInfo for all AZs in the region
	awsEnv.ZoneInfo = lo.Map(lo.Must(awsEnv.EC2API.DescribeAvailabilityZones(&ec2.DescribeAvailabilityZonesInput{})).AvailabilityZones, func(zone *ec2.AvailabilityZone, _ int) ZoneInfo {
		return ZoneInfo{
			Zone:     lo.FromPtr(zone.ZoneName),
			ZoneID:   lo.FromPtr(zone.ZoneId),
			ZoneType: lo.FromPtr(zone.ZoneType),
		}
	})
	return awsEnv
}

func GetTimeStreamAPI(session *session.Session) timestreamwriteiface.TimestreamWriteAPI {
	if lo.Must(env.GetBool("ENABLE_METRICS", false)) {
		By("enabling metrics firing for this suite")
		return timestreamwrite.New(session, &aws.Config{Region: aws.String(env.GetString("METRICS_REGION", metricsDefaultRegion))})
	}
	return &NoOpTimeStreamAPI{}
}

func (env *Environment) DefaultEC2NodeClass() *v1.EC2NodeClass {
	nodeClass := test.EC2NodeClass()
	nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2023@latest"}}
	nodeClass.Spec.Tags = map[string]string{
		"testing/cluster": env.ClusterName,
	}
	nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
		{
			Tags: map[string]string{"karpenter.sh/discovery": env.ClusterName},
		},
	}
	nodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
		{
			Tags: map[string]string{"karpenter.sh/discovery": env.ClusterName},
		},
	}
	if env.PrivateCluster {
		nodeClass.Spec.Role = ""
		nodeClass.Spec.InstanceProfile = lo.ToPtr(fmt.Sprintf("KarpenterNodeInstanceProfile-%s", env.ClusterName))
		return nodeClass
	}
	nodeClass.Spec.Role = fmt.Sprintf("KarpenterNodeRole-%s", env.ClusterName)
	return nodeClass
}
