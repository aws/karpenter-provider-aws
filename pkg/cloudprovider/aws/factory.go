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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/aws/aws-sdk-go/service/ssm"
	provisioningv1alpha1 "github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/cloudprovider/aws/fleet"
	"github.com/awslabs/karpenter/pkg/utils/log"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Factory struct {
	AutoscalingClient autoscalingiface.AutoScalingAPI
	sqs               sqsiface.SQSAPI
	eks               eksiface.EKSAPI
	ec2               ec2iface.EC2API
	FleetFactory      *fleet.Factory
	kubeClient        client.Client
}

func NewFactory(options cloudprovider.Options) *Factory {
	sess := withRegion(session.Must(session.NewSession(&aws.Config{STSRegionalEndpoint: endpoints.RegionalSTSEndpoint})))
	EC2 := ec2.New(sess)
	return &Factory{
		AutoscalingClient: autoscaling.New(sess),
		eks:               eks.New(sess),
		sqs:               sqs.New(sess),
		ec2:               EC2,
		FleetFactory:      fleet.NewFactory(EC2, iam.New(sess), ssm.New(sess), options.Client, options.ClientSet),
		kubeClient:        options.Client,
	}
}

func (f *Factory) CapacityFor(spec *provisioningv1alpha1.ProvisionerSpec) cloudprovider.Capacity {
	return f.FleetFactory.For(spec)
}

func withRegion(sess *session.Session) *session.Session {
	region, err := ec2metadata.New(sess).Region()
	log.PanicIfError(err, "failed to call the metadata server's region API")
	sess.Config.Region = aws.String(region)
	return sess
}
