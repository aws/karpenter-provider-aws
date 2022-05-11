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
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"sort"
	"strings"
	"sync"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
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

func (e EKS) Script() (string, error) {
	var caBundleArg string
	if e.CABundle != nil {
		caBundleArg = fmt.Sprintf("--b64-cluster-ca '%s'", *e.CABundle)
	}
	var userData bytes.Buffer
	userData.WriteString("#!/bin/bash -xe\n")
	userData.WriteString("exec > >(tee /var/log/user-data.log|logger -t user-data -s 2>/dev/console) 2>&1\n")
	// Due to the way bootstrap.sh is written, parameters should not be passed to it with an equal sign
	userData.WriteString(fmt.Sprintf("/etc/eks/bootstrap.sh '%s' --apiserver-endpoint '%s' %s", e.ClusterName, e.ClusterEndpoint, caBundleArg))

	kubeletExtraArgs := strings.Join([]string{e.nodeLabelArg(), e.nodeTaintArg()}, " ")

	if !e.AWSENILimitedPodDensity {
		userData.WriteString(" \\\n--use-max-pods false")
		kubeletExtraArgs += " --max-pods=110"
	}
	if e.ContainerRuntime != "" {
		userData.WriteString(fmt.Sprintf(" \\\n--container-runtime %s", e.ContainerRuntime))
	}
	if kubeletExtraArgs = strings.Trim(kubeletExtraArgs, " "); len(kubeletExtraArgs) > 0 {
		userData.WriteString(fmt.Sprintf(" \\\n--kubelet-extra-args '%s'", kubeletExtraArgs))
	}
	if e.KubeletConfig != nil && len(e.KubeletConfig.ClusterDNS) > 0 {
		userData.WriteString(fmt.Sprintf(" \\\n--dns-cluster-ip '%s'", e.KubeletConfig.ClusterDNS[0]))
	}
	userData, err := e.mergeCustomUserData(userData)
	if err != nil {
		return "", err
	}
	// The mime/multipart package adds carriage returns, while the rest of our logic does not. Remove all
	// carriage returns for consistency.
	userDataBytes := bytes.Replace(userData.Bytes(), []byte{13}, []byte{}, -1)
	userDataString := base64.StdEncoding.EncodeToString(userDataBytes)
	return userDataString, nil
}

func (e EKS) nodeTaintArg() string {
	nodeTaintsArg := ""
	taintStrings := []string{}
	var once sync.Once
	for _, taint := range e.Taints {
		once.Do(func() { nodeTaintsArg = "--register-with-taints=" })
		taintStrings = append(taintStrings, fmt.Sprintf("%s=%s:%s", taint.Key, taint.Value, taint.Effect))
	}
	return fmt.Sprintf("%s%s", nodeTaintsArg, strings.Join(taintStrings, ","))
}

func (e EKS) nodeLabelArg() string {
	nodeLabelArg := ""
	labelStrings := []string{}
	var once sync.Once
	keys := make([]string, 0, len(e.Labels))
	for k := range e.Labels {
		keys = append(keys, k)
	}
	sort.Strings(keys) // ensures this list is deterministic, for easy testing.
	for _, key := range keys {
		if v1alpha5.LabelDomainExceptions.Has(key) {
			continue
		}
		once.Do(func() { nodeLabelArg = "--node-labels=" })
		labelStrings = append(labelStrings, fmt.Sprintf("%s=%v", key, e.Labels[key]))
	}
	return fmt.Sprintf("%s%s", nodeLabelArg, strings.Join(labelStrings, ","))
}

func (e EKS) mergeCustomUserData(userData bytes.Buffer) (bytes.Buffer, error) {
	var outputBuffer bytes.Buffer
	writer := multipart.NewWriter(&outputBuffer)
	if err := writer.SetBoundary(Boundary); err != nil {
		return outputBuffer, fmt.Errorf("defining boundary for merged user data %w", err)
	}
	outputBuffer.WriteString(MIMEVersionHeader + "\n")
	outputBuffer.WriteString(fmt.Sprintf(MIMEContentTypeHeaderTemplate, Boundary) + "\n\n")
	// Step 1 - Copy over customer bootstrapping
	if err := copyCustomUserDataParts(writer, e.Options.CustomUserData); err != nil {
		return outputBuffer, err
	}
	// Step 2 - Add Karpenter's bootstrapping logic
	shellScriptContentHeader := textproto.MIMEHeader{"Content-Type": []string{"text/x-shellscript; charset=\"us-ascii\""}}
	partWriter, err := writer.CreatePart(shellScriptContentHeader)
	if err != nil {
		return outputBuffer, fmt.Errorf("unable to add Karpenter managed user data %w", err)
	}
	_, err = partWriter.Write(userData.Bytes())
	if err != nil {
		return outputBuffer, fmt.Errorf("unable to create merged user data content %w", err)
	}
	writer.Close()
	return outputBuffer, nil
}

func copyCustomUserDataParts(writer *multipart.Writer, customUserData *string) error {
	if customUserData == nil || *customUserData == "" {
		// No custom user data specified, so nothing to copy over.
		return nil
	}
	reader, err := getMultiPartReader(*customUserData)
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
