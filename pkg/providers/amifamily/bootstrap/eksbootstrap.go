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

package bootstrap

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"net/textproto"
	"strings"

	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

type EKS struct {
	Options
	ContainerRuntime string
}

const (
	Boundary                      = "//"
	MIMEVersionHeader             = "MIME-Version: 1.0"
	MIMEContentTypeHeaderTemplate = "Content-Type: multipart/mixed; boundary=\"%s\""
)

func (e EKS) Script(ctx context.Context) (string, error) {
	userData, err := e.mergeCustomUserData(lo.Compact([]string{lo.FromPtr(e.CustomUserData), e.eksBootstrapScript()})...)
	if err != nil {
		return "", err
	}
	// The mime/multipart package adds carriage returns, while the rest of our logic does not. Remove all
	// carriage returns for consistency.
	return base64.StdEncoding.EncodeToString([]byte(strings.ReplaceAll(userData, "\r", ""))), nil
}

//nolint:gocyclo
func (e EKS) eksBootstrapScript() string {
	var caBundleArg string
	if e.CABundle != nil {
		caBundleArg = fmt.Sprintf("--b64-cluster-ca '%s'", *e.CABundle)
	}
	var userData bytes.Buffer
	userData.WriteString("#!/bin/bash -xe\n")
	userData.WriteString("exec > >(tee /var/log/user-data.log|logger -t user-data -s 2>/dev/console) 2>&1\n")
	// Due to the way bootstrap.sh is written, parameters should not be passed to it with an equal sign
	userData.WriteString(fmt.Sprintf("/etc/eks/bootstrap.sh '%s' --apiserver-endpoint '%s' %s", e.ClusterName, e.ClusterEndpoint, caBundleArg))

	if e.isIPv6() {
		userData.WriteString(" \\\n--ip-family ipv6")
	}
	if e.KubeletConfig != nil && len(e.KubeletConfig.ClusterDNS) > 0 {
		userData.WriteString(fmt.Sprintf(" \\\n--dns-cluster-ip '%s'", e.KubeletConfig.ClusterDNS[0]))
	}
	if e.KubeletConfig != nil && e.KubeletConfig.MaxPods != nil {
		userData.WriteString(" \\\n--use-max-pods false")
	}
	if args := e.kubeletExtraArgs(); len(args) > 0 {
		userData.WriteString(fmt.Sprintf(" \\\n--kubelet-extra-args '%s'", strings.Join(args, " ")))
	}
	if lo.FromPtr(e.InstanceStorePolicy) == v1.InstanceStorePolicyRAID0 {
		userData.WriteString(" \\\n--local-disks raid0")
	}
	return userData.String()
}

func (e EKS) mergeCustomUserData(userDatas ...string) (string, error) {
	var outputBuffer bytes.Buffer
	writer := multipart.NewWriter(&outputBuffer)
	if err := writer.SetBoundary(Boundary); err != nil {
		return "", fmt.Errorf("defining boundary for merged user data %w", err)
	}
	outputBuffer.WriteString(MIMEVersionHeader + "\n")
	outputBuffer.WriteString(fmt.Sprintf(MIMEContentTypeHeaderTemplate, Boundary) + "\n\n")
	for _, userData := range userDatas {
		mimedUserData, err := e.mimeify(userData)
		if err != nil {
			return "", err
		}
		if err := copyCustomUserDataParts(writer, mimedUserData); err != nil {
			return "", err
		}
	}
	writer.Close()
	return outputBuffer.String(), nil
}

func (e EKS) isIPv6() bool {
	if e.KubeletConfig == nil || len(e.KubeletConfig.ClusterDNS) == 0 {
		return false
	}
	return net.ParseIP(e.KubeletConfig.ClusterDNS[0]).To4() == nil
}

// mimeify returns userData in a mime format
// if the userData passed in is already in a mime format, then the input is returned without modification
func (e EKS) mimeify(customUserData string) (string, error) {
	if strings.HasPrefix(strings.TrimSpace(customUserData), "MIME-Version:") ||
		strings.HasPrefix(strings.TrimSpace(customUserData), "Content-Type:") {
		return customUserData, nil
	}
	var outputBuffer bytes.Buffer
	writer := multipart.NewWriter(&outputBuffer)
	outputBuffer.WriteString(MIMEVersionHeader + "\n")
	outputBuffer.WriteString(fmt.Sprintf(MIMEContentTypeHeaderTemplate, writer.Boundary()) + "\n\n")
	partWriter, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Type": []string{`text/x-shellscript; charset="us-ascii"`},
	})
	if err != nil {
		return "", fmt.Errorf("creating multi-part section from custom user-data: %w", err)
	}
	_, err = partWriter.Write([]byte(customUserData))
	if err != nil {
		return "", fmt.Errorf("writing custom user-data input: %w", err)
	}
	writer.Close()
	return outputBuffer.String(), nil
}

// copyCustomUserDataParts reads the mime parts in the userData passed in and writes
// to a new mime part in the passed in writer.
func copyCustomUserDataParts(writer *multipart.Writer, customUserData string) error {
	if customUserData == "" {
		// No custom user data specified, so nothing to copy over.
		return nil
	}
	reader, err := getMultiPartReader(customUserData)
	if err != nil {
		return fmt.Errorf("parsing custom user data input %w", err)
	}
	for {
		p, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("parsing custom user data input %w", err)
		}
		slurp, err := io.ReadAll(p)
		if err != nil {
			return fmt.Errorf("parsing custom user data input %w", err)
		}
		partWriter, err := writer.CreatePart(p.Header)
		if err != nil {
			return fmt.Errorf("parsing custom user data input %w", err)
		}
		_, err = partWriter.Write(slurp)
		if err != nil {
			return fmt.Errorf("parsing custom user data input %w", err)
		}
	}
	return nil
}

func getMultiPartReader(userData string) (*multipart.Reader, error) {
	mailMsg, err := mail.ReadMessage(strings.NewReader(userData))
	if err != nil {
		return nil, fmt.Errorf("unreadable user data %w", err)
	}
	mediaType, params, err := mime.ParseMediaType(mailMsg.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("user data does not define a content-type header %w", err)
	}
	if !strings.HasPrefix(mediaType, "multipart/") {
		return nil, fmt.Errorf("user data is not in multipart MIME format")
	}
	return multipart.NewReader(mailMsg.Body, params["boundary"]), nil
}
