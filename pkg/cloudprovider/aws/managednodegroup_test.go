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
*/package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
)

type mockedUpdateManagedNodeGroup struct {
	eksiface.EKSAPI
	Output eks.UpdateNodegroupConfigOutput
	Error  error
}

func (m mockedUpdateManagedNodeGroup) UpdateNodegroupConfig(*eks.UpdateNodegroupConfigInput) (*eks.UpdateNodegroupConfigOutput, error) {
	return &m.Output, m.Error
}

func TestUpdateManagedNodeGroupSuccess(t *testing.T) {
	client := mockedUpdateManagedNodeGroup{
		Output: eks.UpdateNodegroupConfigOutput{},
	}
	asg := &ManagedNodeGroup{
		Client:      client,
		ClusterName: "spatula",
		GroupName:   "dog",
	}
	got := asg.SetReplicas(23)
	if got != nil {
		t.Errorf("SetReplicas(23) = %v; want nil", got)
	}
}
