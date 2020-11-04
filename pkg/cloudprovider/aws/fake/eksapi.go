package fake

import (
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
)

type EKSAPI struct {
	eksiface.EKSAPI
	UpdateOutput   eks.UpdateNodegroupConfigOutput
	DescribeOutput eks.DescribeNodegroupOutput
	WantErr        error
}

func (m EKSAPI) UpdateNodegroupConfig(*eks.UpdateNodegroupConfigInput) (*eks.UpdateNodegroupConfigOutput, error) {
	return &m.UpdateOutput, m.WantErr
}

func (m EKSAPI) DescribeNodegroup(*eks.DescribeNodegroupInput) (*eks.DescribeNodegroupOutput, error) {
	return &m.DescribeOutput, m.WantErr
}
