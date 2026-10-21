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

package common

import (
	"fmt"
	"os"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo/v2"
)

type ManagedController struct {
	Name     string
	Settings map[string]string
	RoleName string
}

func managedControllerFromEnv() *ManagedController {
	name, ok := os.LookupEnv("KARPENTER_MANAGED_CONTROLLER")
	if !ok {
		return nil
	}
	m := &ManagedController{Name: name, Settings: map[string]string{}, RoleName: os.Getenv("KARPENTER_MANAGED_ROLE_NAME")}
	for _, kv := range strings.Split(os.Getenv("KARPENTER_MANAGED_SETTINGS"), ",") {
		if k, v, ok := strings.Cut(kv, "="); ok {
			m.Settings[k] = v
		}
	}
	return m
}

func (m *ManagedController) envVars() []corev1.EnvVar {
	res := make([]corev1.EnvVar, 0, len(m.Settings))
	for k, v := range m.Settings {
		res = append(res, corev1.EnvVar{Name: k, Value: v})
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Name < res[j].Name })
	return res
}
func (env *Environment) RequireInClusterController(need string) {
	GinkgoHelper()
	if env.ManagedController != nil {
		Skip(fmt.Sprintf("[unsupported:in-cluster-controller] %s; %s runs karpenter outside the cluster", need, env.ManagedController.Name))
	}
}
func (env *Environment) skipUnlessManagedSettings(vars []corev1.EnvVar, removed bool) {
	GinkgoHelper()
	for _, v := range vars {
		got, ok := env.ManagedController.Settings[v.Name]
		if removed && !ok || !removed && ok && got == v.Value {
			continue
		}
		Skip(fmt.Sprintf("[unsupported:controller-settings] needs %s=%q; %s has %q", v.Name, ternary(removed, "<unset>", v.Value), env.ManagedController.Name, got))
	}
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
