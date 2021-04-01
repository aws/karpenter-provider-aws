package packing

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func TestSortByResources(t *testing.T) {
	input := &ec2.DescribeInstanceTypesInput{}
	ec2api := ec2.New(session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})))
	instances := []*Instance{}
	if err := ec2api.DescribeInstanceTypesPages(input, func(output *ec2.DescribeInstanceTypesOutput, last bool) bool {
		for _, instance := range output.InstanceTypes {
			instances = append(instances, &Instance{InstanceTypeInfo: *instance})
		}
		return true
	}); err != nil {
		t.Fatalf("Error while describing instance types: %v", err.Error())
	}

	sortByResources(instances)

	for i, instance := range instances {
		fmt.Printf("%d: %s (vcpus: %d, mem: %f)\n", i, *instance.InstanceType, *instance.VCpuInfo.DefaultVCpus, float64(*instance.MemoryInfo.SizeInMiB)/1024)
	}
	t.FailNow()
}
