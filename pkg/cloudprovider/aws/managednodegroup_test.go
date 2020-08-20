package aws

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
		Client: client,
		Ident: ManagedNodeGroupIdentifier{
			Name:    "spatula",
			Cluster: "dog",
		},
	}
	if err := asg.SetReplicas(23); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
