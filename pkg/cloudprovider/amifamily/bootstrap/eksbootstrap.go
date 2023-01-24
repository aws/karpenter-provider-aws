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
	"net"
	"net/mail"
	"net/textproto"
	"sort"
	"strings"
	"sync"

	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/ptr"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/utils/resources"
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

//nolint:gocyclo
func (e EKS) Script() (string, error) {
	var caBundleArg string
	if e.CABundle != nil {
		caBundleArg = fmt.Sprintf("--b64-cluster-ca '%s'", *e.CABundle)
	}
	var userData bytes.Buffer
	var kubeletExtraArgs strings.Builder
	userData.WriteString("#!/bin/bash -xe\n")
	userData.WriteString("exec > >(tee /var/log/user-data.log|logger -t user-data -s 2>/dev/console) 2>&1\n")
	// Due to the way bootstrap.sh is written, parameters should not be passed to it with an equal sign
	userData.WriteString(fmt.Sprintf("/etc/eks/bootstrap.sh '%s' --apiserver-endpoint '%s' %s", e.ClusterName, e.ClusterEndpoint, caBundleArg))

	kubeletExtraArgs.WriteString(strings.Join([]string{e.nodeLabelArg(), e.nodeTaintArg()}, " "))

	if e.isIPv6() {
		userData.WriteString(" \\\n--ip-family ipv6")
	}
	if e.KubeletConfig != nil && e.KubeletConfig.MaxPods != nil {
		userData.WriteString(" \\\n--use-max-pods false")
		kubeletExtraArgs.WriteString(fmt.Sprintf(" --max-pods=%d", ptr.Int32Value(e.KubeletConfig.MaxPods)))
	} else if !e.AWSENILimitedPodDensity {
		userData.WriteString(" \\\n--use-max-pods false")
		kubeletExtraArgs.WriteString(" --max-pods=110")
	}
	if e.KubeletConfig != nil && e.KubeletConfig.PodsPerCore != nil {
		kubeletExtraArgs.WriteString(fmt.Sprintf(" --pods-per-core=%d", ptr.Int32Value(e.KubeletConfig.PodsPerCore)))
	}

	if e.KubeletConfig != nil {
		// We have to convert some of these maps so that their values return the correct string
		kubeletExtraArgs.WriteString(joinParameterArgs("--system-reserved", resources.StringMap(e.KubeletConfig.SystemReserved), "="))
		kubeletExtraArgs.WriteString(joinParameterArgs("--kube-reserved", resources.StringMap(e.KubeletConfig.KubeReserved), "="))
		kubeletExtraArgs.WriteString(joinParameterArgs("--eviction-hard", e.KubeletConfig.EvictionHard, "<"))
		kubeletExtraArgs.WriteString(joinParameterArgs("--eviction-soft", e.KubeletConfig.EvictionSoft, "<"))
		kubeletExtraArgs.WriteString(joinParameterArgs("--eviction-soft-grace-period", lo.MapValues(e.KubeletConfig.EvictionSoftGracePeriod, func(v metav1.Duration, _ string) string { return v.Duration.String() }), "="))

		if e.KubeletConfig.EvictionMaxPodGracePeriod != nil {
			kubeletExtraArgs.WriteString(fmt.Sprintf(" --eviction-max-pod-grace-period=%d", ptr.Int32Value(e.KubeletConfig.EvictionMaxPodGracePeriod)))
		}

		if e.KubeletConfig.ImageGCHighThresholdPercent != nil {
			kubeletExtraArgs.WriteString(fmt.Sprintf(" --image-gc-high-threshold=%d", ptr.Int32Value(e.KubeletConfig.ImageGCHighThresholdPercent)))
		}

		if e.KubeletConfig.ImageGCLowThresholdPercent != nil {
			kubeletExtraArgs.WriteString(fmt.Sprintf(" --image-gc-low-threshold=%d", ptr.Int32Value(e.KubeletConfig.ImageGCLowThresholdPercent)))
		}
	}
	if e.ContainerRuntime != "" {
		userData.WriteString(fmt.Sprintf(" \\\n--container-runtime %s", e.ContainerRuntime))
	}
	if kubeletExtraArgsStr := strings.Trim(kubeletExtraArgs.String(), " "); len(kubeletExtraArgsStr) > 0 {
		userData.WriteString(fmt.Sprintf(" \\\n--kubelet-extra-args '%s'", kubeletExtraArgsStr))
	}
	if e.KubeletConfig != nil && len(e.KubeletConfig.ClusterDNS) > 0 {
		userData.WriteString(fmt.Sprintf(" \\\n--dns-cluster-ip '%s'", e.KubeletConfig.ClusterDNS[0]))
	}
	userDataMerged, err := e.mergeCustomUserData(&userData)
	if err != nil {
		return "", err
	}
	// The mime/multipart package adds carriage returns, while the rest of our logic does not. Remove all
	// carriage returns for consistency.
	userDataBytes := bytes.Replace(userDataMerged.Bytes(), []byte{13}, []byte{}, -1)
	return base64.StdEncoding.EncodeToString(userDataBytes), nil
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
	keys := lo.Keys(e.Labels)
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

// joinParameterArgs joins a map of keys and values by their separator. The separator will sit between the
// arguments in a comma-separated list i.e. arg1<sep>val1,arg2<sep>val2
func joinParameterArgs[K comparable, V any](name string, m map[K]V, separator string) string {
	var args []string

	for k, v := range m {
		args = append(args, fmt.Sprintf("%v%s%v", k, separator, v))
	}
	if len(args) > 0 {
		return fmt.Sprintf(" %s=%s", name, strings.Join(args, ","))
	}
	return ""
}

func (e EKS) mergeCustomUserData(userData *bytes.Buffer) (*bytes.Buffer, error) {
	var outputBuffer bytes.Buffer
	writer := multipart.NewWriter(&outputBuffer)
	if err := writer.SetBoundary(Boundary); err != nil {
		return nil, fmt.Errorf("defining boundary for merged user data %w", err)
	}
	outputBuffer.WriteString(MIMEVersionHeader + "\n")
	outputBuffer.WriteString(fmt.Sprintf(MIMEContentTypeHeaderTemplate, Boundary) + "\n\n")
	// Step 1 - Copy over customer bootstrapping
	if err := copyCustomUserDataParts(writer, e.Options.CustomUserData); err != nil {
		return nil, err
	}
	// Step 2 - Add Karpenter's bootstrapping logic
	shellScriptContentHeader := textproto.MIMEHeader{"Content-Type": []string{"text/x-shellscript; charset=\"us-ascii\""}}
	partWriter, err := writer.CreatePart(shellScriptContentHeader)
	if err != nil {
		return nil, fmt.Errorf("unable to add Karpenter managed user data %w", err)
	}
	_, err = partWriter.Write(userData.Bytes())
	if err != nil {
		return nil, fmt.Errorf("unable to create merged user data content %w", err)
	}
	writer.Close()
	return &outputBuffer, nil
}

func (e EKS) isIPv6() bool {
	if e.KubeletConfig == nil || len(e.KubeletConfig.ClusterDNS) == 0 {
		return false
	}
	return net.ParseIP(e.KubeletConfig.ClusterDNS[0]).To4() == nil
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
