package imds

import (
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
)

var client *imds.Client

func init() {
	client = imds.New(imds.Options{})
}

type IMDSProperty string

const (
	ServicesDomain IMDSProperty = "services/domain"
)

func GetProperty(prop IMDSProperty) (string, error) {
	bytes, err := GetPropertyBytes(prop)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func GetPropertyBytes(prop IMDSProperty) ([]byte, error) {
	res, err := client.GetMetadata(context.TODO(), &imds.GetMetadataInput{Path: string(prop)})
	if err != nil {
		return nil, err
	}
	return io.ReadAll(res.Content)
}
