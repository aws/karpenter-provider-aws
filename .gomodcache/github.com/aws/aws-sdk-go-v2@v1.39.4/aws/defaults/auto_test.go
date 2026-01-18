package defaults

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"strconv"
	"testing"
)

func TestDetermineDefaultsMode(t *testing.T) {
	cases := []struct {
		Region      string
		GOOS        string
		Environment aws.RuntimeEnvironment
		Expected    aws.DefaultsMode
	}{
		{
			Region: "us-east-1",
			GOOS:   "ios",
			Environment: aws.RuntimeEnvironment{
				EnvironmentIdentifier: aws.ExecutionEnvironmentID("AWS_Lambda_java8"),
				Region:                "us-east-1",
			},
			Expected: aws.DefaultsModeMobile,
		},
		{
			Region: "us-east-1",
			GOOS:   "android",
			Environment: aws.RuntimeEnvironment{
				EnvironmentIdentifier: aws.ExecutionEnvironmentID("AWS_Lambda_java8"),
				Region:                "us-east-1",
			},
			Expected: aws.DefaultsModeMobile,
		},
		{
			Region: "us-east-1",
			Environment: aws.RuntimeEnvironment{
				EnvironmentIdentifier: aws.ExecutionEnvironmentID("AWS_Lambda_java8"),
				Region:                "us-east-1",
			},
			Expected: aws.DefaultsModeInRegion,
		},
		{
			Region: "us-east-1",
			Environment: aws.RuntimeEnvironment{
				EnvironmentIdentifier: aws.ExecutionEnvironmentID("AWS_Lambda_java8"),
				Region:                "us-west-2",
			},
			Expected: aws.DefaultsModeCrossRegion,
		},
		{
			Region: "us-east-1",
			Environment: aws.RuntimeEnvironment{
				Region:                    "us-east-1",
				EC2InstanceMetadataRegion: "us-east-1",
			},
			Expected: aws.DefaultsModeInRegion,
		},
		{
			Region: "us-east-1",
			Environment: aws.RuntimeEnvironment{
				EC2InstanceMetadataRegion: "us-west-2",
			},
			Expected: aws.DefaultsModeCrossRegion,
		},
		{
			Region:      "us-west-2",
			Environment: aws.RuntimeEnvironment{},
			Expected:    aws.DefaultsModeStandard,
		},
	}

	for i, tt := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			if len(tt.GOOS) > 0 {
				orig := getGOOS
				getGOOS = func() string {
					return tt.GOOS
				}
				defer func() {
					getGOOS = orig
				}()
			}
			got := ResolveDefaultsModeAuto(tt.Region, tt.Environment)
			if got != tt.Expected {
				t.Errorf("expect %v, got %v", tt.Expected, got)
			}
		})
	}
}
