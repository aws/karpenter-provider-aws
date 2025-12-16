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
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	admapi "github.com/awslabs/amazon-eks-ami/nodeadm/api"
	admv1alpha1 "github.com/awslabs/amazon-eks-ami/nodeadm/api/v1alpha1"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/yaml"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily/bootstrap/mime"
)

type Nodeadm struct {
	Options
}

func (n Nodeadm) Script(_ context.Context) (string, error) {
	nodeConfigYAML, err := n.getNodeConfigYAML()
	if err != nil {
		return "", fmt.Errorf("generating NodeConfig, %w", err)
	}
	customEntries, err := n.parseUserData()
	if err != nil {
		return "", fmt.Errorf("parsing custom UserData, %w", err)
	}
	mimeArchive := mime.Archive(append(customEntries, mime.Entry{
		ContentType: mime.ContentTypeNodeConfig,
		Content:     nodeConfigYAML,
	}))
	userData, err := mimeArchive.Serialize()
	if err != nil {
		return "", err
	}
	return userData, nil
}

// getNodeConfigYAML returns the Karpenter generated NodeConfig YAML object serialized as a string
func (n Nodeadm) getNodeConfigYAML() (string, error) {
	config := &admv1alpha1.NodeConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       admapi.KindNodeConfig,
			APIVersion: admv1alpha1.GroupVersion.String(),
		},
		Spec: admv1alpha1.NodeConfigSpec{
			Cluster: admv1alpha1.ClusterDetails{
				Name:              n.ClusterName,
				APIServerEndpoint: n.ClusterEndpoint,
				CIDR:              lo.FromPtr(n.ClusterCIDR),
			},
		},
	}
	if lo.FromPtr(n.CABundle) != "" {
		ca, err := base64.StdEncoding.DecodeString(*n.CABundle)
		if err != nil {
			return "", fmt.Errorf("decoding CABundle, %w", err)
		}
		config.Spec.Cluster.CertificateAuthority = ca
	}
	if cidr := lo.FromPtr(n.ClusterCIDR); cidr != "" {
		config.Spec.Cluster.CIDR = cidr
	} else {
		return "", cloudprovider.NewNodeClassNotReadyError(fmt.Errorf("resolving cluster CIDR"))
	}
	if lo.FromPtr(n.InstanceStorePolicy) == v1.InstanceStorePolicyRAID0 {
		config.Spec.Instance.LocalStorage.Strategy = admv1alpha1.LocalStorageRAID0
	}
	inlineConfig, err := n.generateInlineKubeletConfiguration()
	if err != nil {
		return "", err
	}
	config.Spec.Kubelet.Config = inlineConfig
	if arg := n.nodeLabelArg(); arg != "" {
		config.Spec.Kubelet.Flags = []string{arg}
	}

	// Convert to YAML at the end for improved legibility.
	configYAML, err := yaml.Marshal(config)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("# Karpenter Generated NodeConfig\n%s", string(configYAML)), nil
}

// generateInlineKubeletConfiguration returns a serialized form of the KubeletConfiguration specified by the Nodeadm
// options, for use with nodeadm's NodeConfig struct.
func (n Nodeadm) generateInlineKubeletConfiguration() (map[string]runtime.RawExtension, error) {
	kubeConfigJSON, err := json.Marshal(n.KubeletConfig)
	if err != nil {
		return nil, err
	}
	kubeConfigMap := map[string]runtime.RawExtension{}
	err = json.Unmarshal(kubeConfigJSON, &kubeConfigMap)
	if err != nil {
		return nil, err
	}
	kubeConfigMap["registerWithTaints"] = runtime.RawExtension{
		Raw: lo.Must(json.Marshal(n.Taints)),
	}
	return kubeConfigMap, nil
}

// parseUserData returns a slice of MIMEEntrys corresponding to each entry in the custom UserData. If the custom
// UserData is not a MIME multi-part archive, the content type will be detected (NodeConfig or shell) and an entry
// will be created.
func (n Nodeadm) parseUserData() ([]mime.Entry, error) {
	userData := lo.FromPtr(n.CustomUserData)
	if userData == "" {
		return nil, nil
	}
	if strings.HasPrefix(strings.TrimSpace(userData), "MIME-Version:") ||
		strings.HasPrefix(strings.TrimSpace(userData), "Content-Type:") {
		archive, err := mime.NewArchive(userData)
		if err != nil {
			return nil, err
		}
		return archive, nil
	}
	// Fallback to YAML or shall script if UserData is not in MIME format. Determine the content type for the
	// generated MIME header depending on the type of the custom UserData.
	if err := yaml.Unmarshal([]byte(*n.CustomUserData), lo.ToPtr(map[string]interface{}{})); err == nil {
		return []mime.Entry{{
			ContentType: mime.ContentTypeNodeConfig,
			Content:     userData,
		}}, nil
	}
	return []mime.Entry{{
		ContentType: mime.ContentTypeShellScript,
		Content:     userData,
	}}, nil
}
