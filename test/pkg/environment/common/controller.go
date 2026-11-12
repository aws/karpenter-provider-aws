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
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo/v2"
)

type ExternalController struct {
	Name     string
	Settings map[string]string
	RoleName string
}

func externalControllerFromEnv() *ExternalController {
	name, ok := os.LookupEnv("KARPENTER_EXTERNAL_CONTROLLER")
	if !ok {
		return nil
	}
	m := &ExternalController{Name: name, Settings: map[string]string{}, RoleName: os.Getenv("KARPENTER_EXTERNAL_ROLE_NAME")}
	if v := os.Getenv("KARPENTER_EXTERNAL_SETTINGS"); v != "" {
		if err := json.Unmarshal([]byte(v), &m.Settings); err != nil {
			panic(fmt.Sprintf("KARPENTER_EXTERNAL_SETTINGS must be a JSON object of strings, e.g. {\"FEATURE_GATES\":\"NodeRepair=true,StaticCapacity=true\"}: %v", err))
		}
	}
	return m
}

func (m *ExternalController) envVars() []corev1.EnvVar {
	res := make([]corev1.EnvVar, 0, len(m.Settings))
	for k, v := range m.Settings {
		res = append(res, corev1.EnvVar{Name: k, Value: v})
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Name < res[j].Name })
	return res
}

func (env *Environment) RequireInClusterController(need string) {
	GinkgoHelper()
	if env.ExternalController != nil {
		Skip(fmt.Sprintf("[unsupported:in-cluster-controller] %s; %s runs karpenter outside the cluster", need, env.ExternalController.Name))
	}
}

func (env *Environment) skipUnlessControllerSettings(vars []corev1.EnvVar, removed bool) {
	GinkgoHelper()
	for _, v := range vars {
		got, ok := env.ExternalController.Settings[v.Name]
		if removed {
			if ok {
				Skip(fmt.Sprintf("[unsupported:controller-settings] needs %s unset; %s has %q", v.Name, env.ExternalController.Name, got))
			}
			continue
		}
		if !ok {
			Skip(fmt.Sprintf("[unsupported:controller-settings] needs %s=%q; %s doesn't set it", v.Name, v.Value, env.ExternalController.Name))
		}
		if unmet := unmetSetting(got, v.Value); len(unmet) > 0 {
			Skip(fmt.Sprintf("[unsupported:controller-settings] needs %s %s; %s has %q", v.Name, strings.Join(unmet, ","), env.ExternalController.Name, got))
		}
	}
}

func unmetSetting(have, want string) []string {
	if have == want {
		return nil
	}
	h, hok := keyValues(have)
	w, wok := keyValues(want)
	if !hok || !wok {
		return []string{fmt.Sprintf("%q", want)}
	}
	var unmet []string
	for _, k := range lo.Keys(w) {
		if hv, ok := h[k]; !ok || !sameValue(hv, w[k]) {
			unmet = append(unmet, k+"="+w[k])
		}
	}
	sort.Strings(unmet)
	return unmet
}

func keyValues(s string) (map[string]string, bool) {
	res := map[string]string{}
	for _, kv := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(kv), "=")
		if !ok || k == "" {
			return nil, false
		}
		res[k] = v
	}
	return res, true
}

func sameValue(a, b string) bool {
	if a == b {
		return true
	}
	x, errA := strconv.ParseBool(a)
	y, errB := strconv.ParseBool(b)
	return errA == nil && errB == nil && x == y
}
