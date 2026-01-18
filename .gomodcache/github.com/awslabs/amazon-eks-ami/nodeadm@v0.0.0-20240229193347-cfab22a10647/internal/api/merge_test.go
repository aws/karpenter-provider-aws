package api

import (
	"reflect"
	"testing"
)

func toInlineDocumentMust(m map[string]interface{}) InlineDocument {
	d, err := toInlineDocument(m)
	if err != nil {
		panic(err)
	}
	return d
}

func TestMerge(t *testing.T) {
	var tests = []struct {
		name         string
		baseSpec     NodeConfigSpec
		patchSpec    NodeConfigSpec
		expectedSpec NodeConfigSpec
	}{
		{
			name: "merge with empty string field",
			baseSpec: NodeConfigSpec{
				Cluster: ClusterDetails{},
			},
			patchSpec: NodeConfigSpec{
				Cluster: ClusterDetails{Name: "override"},
			},
			expectedSpec: NodeConfigSpec{
				Cluster: ClusterDetails{Name: "override"},
			},
		},
		{
			name: "merge with existing string field",
			baseSpec: NodeConfigSpec{
				Cluster: ClusterDetails{Name: "previous"},
			},
			patchSpec: NodeConfigSpec{
				Cluster: ClusterDetails{Name: "next"},
			},
			expectedSpec: NodeConfigSpec{
				Cluster: ClusterDetails{Name: "next"},
			},
		},
		{
			name: "customer overrides orchestrator defaults",
			baseSpec: NodeConfigSpec{
				Cluster: ClusterDetails{
					Name:                 "example",
					APIServerEndpoint:    "http://example.com",
					CertificateAuthority: []byte("example data"),
					CIDR:                 "10.0.0.0/16",
				},
				Kubelet: KubeletOptions{
					Config: toInlineDocumentMust(map[string]interface{}{
						"logging": map[string]interface{}{
							"verbosity": 5,
						},
						"podsPerCore": 20,
					}),
					Flags: []string{
						"--node-labels=nodegroup=example",
						"--register-with-taints=the=taint:NoSchedule",
					},
				},
				Containerd: ContainerdOptions{
					Config: "base",
				},
			},
			patchSpec: NodeConfigSpec{
				Kubelet: KubeletOptions{
					Config: toInlineDocumentMust(map[string]interface{}{
						"logging": map[string]interface{}{
							"verbosity": 2,
						},
						"maxPods": 150,
					}),
					Flags: []string{
						"--node-labels=nodegroup=user-set",
					},
				},
				Containerd: ContainerdOptions{
					Config: "patch",
				},
			},
			expectedSpec: NodeConfigSpec{
				Cluster: ClusterDetails{
					Name:                 "example",
					APIServerEndpoint:    "http://example.com",
					CertificateAuthority: []byte("example data"),
					CIDR:                 "10.0.0.0/16",
				},
				Kubelet: KubeletOptions{
					Config: toInlineDocumentMust(map[string]interface{}{
						"logging": map[string]interface{}{
							"verbosity": 2,
						},
						"maxPods":     150,
						"podsPerCore": 20,
					}),
					Flags: []string{
						"--node-labels=nodegroup=example",
						"--register-with-taints=the=taint:NoSchedule",
						"--node-labels=nodegroup=user-set",
					},
				},
				Containerd: ContainerdOptions{
					Config: "patch",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			baseConfig := NodeConfig{Spec: test.baseSpec}
			patchConfig := NodeConfig{Spec: test.patchSpec}
			if err := baseConfig.Merge(&patchConfig); err != nil {
				t.Error(err)
			}
			expectedConfig := NodeConfig{Spec: test.expectedSpec}
			if !reflect.DeepEqual(expectedConfig, baseConfig) {
				t.Errorf("\nexpected: %+v\n\ngot:       %+v", expectedConfig, baseConfig)
			}
		})
	}
}
