package configprovider

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/awslabs/amazon-eks-ami/nodeadm/api"
	internalapi "github.com/awslabs/amazon-eks-ami/nodeadm/internal/api"
	apibridge "github.com/awslabs/amazon-eks-ami/nodeadm/internal/api/bridge"
)

const (
	contentTypeHeader          = "Content-Type"
	mimeBoundaryParam          = "boundary"
	multipartContentTypePrefix = "multipart/"
	nodeConfigMediaType        = "application/" + api.GroupName
)

type userDataConfigProvider struct {
	imdsClient *imds.Client
}

func NewUserDataConfigProvider() ConfigProvider {
	return &userDataConfigProvider{
		imdsClient: imds.New(imds.Options{}),
	}
}

func (ics *userDataConfigProvider) Provide() (*internalapi.NodeConfig, error) {
	userData, err := ics.getUserData()
	if err != nil {
		return nil, err
	}
	// if the MIME data fails to parse as a multipart document, then fall back
	// to parsing the entire userdata as the node config.
	if multipartReader, err := getMIMEMultipartReader(userData); err == nil {
		config, err := parseMultipart(multipartReader)
		if err != nil {
			return nil, err
		}
		return config, nil
	} else {
		config, err := apibridge.DecodeNodeConfig(userData)
		if err != nil {
			return nil, err
		}
		return config, nil
	}
}

func (ics userDataConfigProvider) getUserData() ([]byte, error) {
	resp, err := ics.imdsClient.GetUserData(context.TODO(), &imds.GetUserDataInput{})
	if err != nil {
		return nil, err
	}
	return io.ReadAll(resp.Content)
}

func getMIMEMultipartReader(data []byte) (*multipart.Reader, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	mediaType, params, err := mime.ParseMediaType(msg.Header.Get(contentTypeHeader))
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(mediaType, multipartContentTypePrefix) {
		return nil, fmt.Errorf("MIME type is not multipart")
	}
	return multipart.NewReader(msg.Body, params[mimeBoundaryParam]), nil
}

func parseMultipart(userDataReader *multipart.Reader) (*internalapi.NodeConfig, error) {
	var nodeConfigs []*internalapi.NodeConfig
	for {
		part, err := userDataReader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if partHeader := part.Header.Get(contentTypeHeader); len(partHeader) > 0 {
			mediaType, _, err := mime.ParseMediaType(partHeader)
			if err != nil {
				return nil, err
			}
			if mediaType == nodeConfigMediaType {
				nodeConfigPart, err := io.ReadAll(part)
				if err != nil {
					return nil, err
				}
				decodedConfig, err := apibridge.DecodeNodeConfig(nodeConfigPart)
				if err != nil {
					return nil, err
				}
				nodeConfigs = append(nodeConfigs, decodedConfig)
			}
		}
	}
	if len(nodeConfigs) > 0 {
		var config = nodeConfigs[0]
		for _, nodeConfig := range nodeConfigs[1:] {
			if err := config.Merge(nodeConfig); err != nil {
				return nil, err
			}
		}
		return config, nil
	} else {
		return nil, fmt.Errorf("Could not find NodeConfig within UserData")
	}
}
