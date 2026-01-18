/*
Copyright 2019 The Kubernetes Authors.

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

package config

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type testCase struct {
	text           string
	context        string
	kubeconfigFlag string
	kubeconfigEnv  []string
	wantHost       string
}

var _ = Describe("Config", func() {

	var dir string

	origRecommendedHomeFile := clientcmd.RecommendedHomeFile

	BeforeEach(func() {
		// create temporary directory for test case
		var err error
		dir, err = os.MkdirTemp("", "cr-test")
		Expect(err).NotTo(HaveOccurred())

		// override $HOME/.kube/config
		clientcmd.RecommendedHomeFile = filepath.Join(dir, ".kubeconfig")
	})

	AfterEach(func() {
		_ = os.Unsetenv(clientcmd.RecommendedConfigPathEnvVar)
		kubeconfig = ""
		clientcmd.RecommendedHomeFile = origRecommendedHomeFile

		err := os.RemoveAll(dir)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("GetConfigWithContext", func() {
		defineTests := func(testCases []testCase) {
			for _, testCase := range testCases {
				tc := testCase
				It(tc.text, func() {
					// set global and environment configs
					setConfigs(tc, dir)

					// run the test
					cfg, err := GetConfigWithContext(tc.context)
					Expect(err).NotTo(HaveOccurred())
					Expect(cfg.Host).To(Equal(tc.wantHost))
					Expect(cfg.QPS).To(Equal(float32(-1)))
				})
			}
		}

		Context("when kubeconfig files don't exist", func() {
			It("should fail", func() {
				err := os.Unsetenv(clientcmd.RecommendedConfigPathEnvVar)
				Expect(err).NotTo(HaveOccurred())

				cfg, err := GetConfigWithContext("")
				Expect(err).To(HaveOccurred())
				Expect(cfg).To(BeNil())
			})
		})

		Context("when in-cluster", func() {
			kubeconfigFiles := map[string]string{
				"kubeconfig-multi-context": genKubeconfig("from-multi-env-1", "from-multi-env-2"),
				".kubeconfig":              genKubeconfig("from-home"),
			}
			BeforeEach(func() {
				err := createFiles(kubeconfigFiles, dir)
				Expect(err).NotTo(HaveOccurred())

				// override in-cluster config loader
				loadInClusterConfig = func() (*rest.Config, error) {
					return &rest.Config{Host: "from-in-cluster"}, nil
				}
			})
			AfterEach(func() { loadInClusterConfig = rest.InClusterConfig })

			testCases := []testCase{
				{
					text:          "should prefer the envvar over the in-cluster config",
					kubeconfigEnv: []string{"kubeconfig-multi-context"},
					wantHost:      "from-multi-env-1",
				},
				{
					text:     "should prefer in-cluster over the recommended home file",
					wantHost: "from-in-cluster",
				},
			}
			defineTests(testCases)
		})

		Context("when outside the cluster", func() {
			kubeconfigFiles := map[string]string{
				"kubeconfig-flag":          genKubeconfig("from-flag"),
				"kubeconfig-multi-context": genKubeconfig("from-multi-env-1", "from-multi-env-2"),
				"kubeconfig-env-1":         genKubeconfig("from-env-1"),
				"kubeconfig-env-2":         genKubeconfig("from-env-2"),
				".kubeconfig":              genKubeconfig("from-home"),
			}
			BeforeEach(func() {
				err := createFiles(kubeconfigFiles, dir)
				Expect(err).NotTo(HaveOccurred())
			})
			testCases := []testCase{
				{
					text:           "should use the --kubeconfig flag",
					kubeconfigFlag: "kubeconfig-flag",
					wantHost:       "from-flag",
				},
				{
					text:          "should use the envvar",
					kubeconfigEnv: []string{"kubeconfig-multi-context"},
					wantHost:      "from-multi-env-1",
				},
				{
					text:     "should use the recommended home file",
					wantHost: "from-home",
				},
				{
					text:           "should prefer the flag over the envvar",
					kubeconfigFlag: "kubeconfig-flag",
					kubeconfigEnv:  []string{"kubeconfig-multi-context"},
					wantHost:       "from-flag",
				},
				{
					text:          "should prefer the envvar over the recommended home file",
					kubeconfigEnv: []string{"kubeconfig-multi-context"},
					wantHost:      "from-multi-env-1",
				},
				{
					text:          "should allow overriding the context",
					context:       "from-multi-env-2",
					kubeconfigEnv: []string{"kubeconfig-multi-context"},
					wantHost:      "from-multi-env-2",
				},
				{
					text:          "should support a multi-value envvar",
					context:       "from-env-2",
					kubeconfigEnv: []string{"kubeconfig-env-1", "kubeconfig-env-2"},
					wantHost:      "from-env-2",
				},
			}
			defineTests(testCases)
		})
	})
})

func setConfigs(tc testCase, dir string) {
	// Set kubeconfig flag value
	if len(tc.kubeconfigFlag) > 0 {
		kubeconfig = filepath.Join(dir, tc.kubeconfigFlag)
	}

	// Set KUBECONFIG env value
	if len(tc.kubeconfigEnv) > 0 {
		kubeconfigEnvPaths := []string{}
		for _, k := range tc.kubeconfigEnv {
			kubeconfigEnvPaths = append(kubeconfigEnvPaths, filepath.Join(dir, k))
		}
		os.Setenv(clientcmd.RecommendedConfigPathEnvVar, strings.Join(kubeconfigEnvPaths, ":"))
	}
}

func createFiles(files map[string]string, dir string) error {
	for path, data := range files {
		if err := os.WriteFile(filepath.Join(dir, path), []byte(data), 0644); err != nil {
			return err
		}
	}
	return nil
}

func genKubeconfig(contexts ...string) string {
	var sb strings.Builder
	sb.WriteString(`---
apiVersion: v1
kind: Config
clusters:
`)
	for _, ctx := range contexts {
		sb.WriteString(`- cluster:
    server: ` + ctx + `
  name: ` + ctx + `
`)
	}
	sb.WriteString("contexts:\n")
	for _, ctx := range contexts {
		sb.WriteString(`- context:
    cluster: ` + ctx + `
    user: ` + ctx + `
  name: ` + ctx + `
`)
	}

	sb.WriteString("users:\n")
	for _, ctx := range contexts {
		sb.WriteString(`- name: ` + ctx + `
`)
	}
	sb.WriteString("preferences: {}\n")
	if len(contexts) > 0 {
		sb.WriteString("current-context: " + contexts[0] + "\n")
	}

	return sb.String()
}
