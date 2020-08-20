package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
)

type mockedUpdateManagedNodeGroup struct {
	eksiface.EKSAPI
	Resp  eks.UpdateNodegroupConfigOutput
	Error error
}

func (m mockedUpdateManagedNodeGroup) UpdateNodegroupConfig(*eks.UpdateNodegroupConfigInput) (*eks.UpdateNodegroupConfigOutput, error) {
	if m.Error != nil {
		return nil, m.Error
	} else {
		return &m.Resp, nil
	}
}

func TestUpdateManagedNodeGroupSuccess(t *testing.T) {
	client := mockedUpdateManagedNodeGroup{
		Resp: eks.UpdateNodegroupConfigOutput{},
	}
	asg := NewManagedNodeGroup(client, "spatula", "dog")
	err := asg.SetReplicas(23)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
