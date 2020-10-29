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
	"io/ioutil"
	"net/http"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/cloudprovider"
	"github.com/ellistarn/karpenter/pkg/cloudprovider/mock"
	nodegroupaws "github.com/ellistarn/karpenter/pkg/cloudprovider/nodegroup/aws"
	queueaws "github.com/ellistarn/karpenter/pkg/cloudprovider/queue/aws"
	"github.com/ellistarn/karpenter/pkg/utils/log"
)

type Factory struct {
	Client autoscalingiface.AutoScalingAPI
}

func NewFactory() *Factory {
	return &Factory{
		Client: autoscaling.New(
			session.Must(session.NewSession(&aws.Config{Region: aws.String(regionOrDie())})),
		),
	}
}

func (f *Factory) NodeGroupFor(spec *v1alpha1.ScalableNodeGroupSpec) cloudprovider.NodeGroup {
	switch spec.Type {
	case v1alpha1.AWSEC2AutoScalingGroup:
		return nodegroupaws.NewAutoScalingGroup(spec.ID)
	case v1alpha1.AWSEKSNodeGroup:
		return nodegroupaws.NewManagedNodeGroup(spec.ID)
	default:
		return mock.NewNotImplementedFactory().NodeGroupFor(spec)
	}
}

func (f *Factory) QueueFor(spec *v1alpha1.QueueSpec) cloudprovider.Queue {
	switch spec.Type {
	case v1alpha1.AWSSQSQueueType:
		return &queueaws.SQSQueue{ARN: spec.ID}
	default:
		return mock.NewNotImplementedFactory().QueueFor(spec)
	}
}

// TODO use aws-sdk APIs
func regionOrDie() string {
	response, err := http.Get("http://169.254.169.254/latest/meta-data/placement/region")
	log.PanicIfError(err, "Failed to call the metadata server's region API")
	region, err := ioutil.ReadAll(response.Body)
	log.PanicIfError(err, "Failed to read response from metadata server")
	return string(region)
}
