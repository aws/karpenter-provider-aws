package awsrulesfn

import (
	"fmt"
	"reflect"
	"testing"
)

func TestParseARN(t *testing.T) {
	cases := []struct {
		input  string
		expect *ARN
	}{
		{
			input:  "invalid",
			expect: nil,
		},
		{
			input:  "arn:nope",
			expect: nil,
		},
		{
			input: "arn:aws:ecr:us-west-2:123456789012:repository/foo/bar",
			expect: &ARN{
				Partition:  "aws",
				Service:    "ecr",
				Region:     "us-west-2",
				AccountId:  "123456789012",
				ResourceId: []string{"repository", "foo", "bar"},
			},
		},
		{
			input: "arn:aws:elasticbeanstalk:us-east-1:123456789012:environment/My App/MyEnvironment",
			expect: &ARN{
				Partition:  "aws",
				Service:    "elasticbeanstalk",
				Region:     "us-east-1",
				AccountId:  "123456789012",
				ResourceId: []string{"environment", "My App", "MyEnvironment"},
			},
		},
		{
			input: "arn:aws:iam::123456789012:user/David",
			expect: &ARN{
				Partition:  "aws",
				Service:    "iam",
				Region:     "",
				AccountId:  "123456789012",
				ResourceId: []string{"user", "David"},
			},
		},
		{
			input: "arn:aws:rds:eu-west-1:123456789012:db:mysql-db",
			expect: &ARN{
				Partition:  "aws",
				Service:    "rds",
				Region:     "eu-west-1",
				AccountId:  "123456789012",
				ResourceId: []string{"db", "mysql-db"},
			},
		},
		{
			input: "arn:aws:s3:::my_corporate_bucket/exampleobject.png",
			expect: &ARN{
				Partition:  "aws",
				Service:    "s3",
				Region:     "",
				AccountId:  "",
				ResourceId: []string{"my_corporate_bucket", "exampleobject.png"},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			actual := ParseARN(c.input)
			if diff := cmpDiff(c.expect, actual); diff != "" {
				t.Errorf("expect ARN match\n%s", diff)
			}
		})
	}
}

func cmpDiff(e, a interface{}) string {
	if !reflect.DeepEqual(e, a) {
		return fmt.Sprintf("%v != %v", e, a)
	}
	return ""
}
