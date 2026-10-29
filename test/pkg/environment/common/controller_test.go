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
	"reflect"
	"testing"
)

func TestExternalControllerFromEnv(t *testing.T) {
	t.Setenv("KARPENTER_EXTERNAL_CONTROLLER", "test-controller")
	t.Setenv("KARPENTER_EXTERNAL_SETTINGS", `{"MIN_VALUES_POLICY":"Strict","FEATURE_GATES":"NodeRepair=true,StaticCapacity=true"}`)
	got := externalControllerFromEnv().Settings
	want := map[string]string{"MIN_VALUES_POLICY": "Strict", "FEATURE_GATES": "NodeRepair=true,StaticCapacity=true"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Settings = %v, want %v", got, want)
	}

	for _, value := range []string{"MIN_VALUES_POLICY=Strict", `{"FEATURE_GATES":true}`} {
		func() {
			t.Setenv("KARPENTER_EXTERNAL_SETTINGS", value)
			defer func() {
				if recover() == nil {
					t.Errorf("KARPENTER_EXTERNAL_SETTINGS=%s didn't panic", value)
				}
			}()
			externalControllerFromEnv()
		}()
	}
}

func TestUnmetSetting(t *testing.T) {
	gates := "SpotToSpotConsolidation=true,NodeRepair=true,ReservedCapacity=true,NodeOverlay=false"
	for _, tc := range []struct {
		have, want string
		unmet      []string
	}{
		{have: "Strict", want: "Strict"},
		{have: "Strict", want: "BestEffort", unmet: []string{`"BestEffort"`}},
		{have: gates, want: "NodeRepair=true"},
		{have: gates, want: "NodeRepair=True,NodeOverlay=false"},
		{have: gates, want: "ReservedCapacity=true,SpotToSpotConsolidation=true,NodeRepair=true,NodeOverlay=false"},
		{have: gates, want: "NodeOverlay=true", unmet: []string{"NodeOverlay=true"}},
		{have: gates, want: "TerminateFirstDrift=true,NodeRepair=true,Drift=true", unmet: []string{"Drift=true", "TerminateFirstDrift=true"}},
		{have: gates, want: "Strict", unmet: []string{`"Strict"`}},
	} {
		if got := unmetSetting(tc.have, tc.want); !reflect.DeepEqual(got, tc.unmet) {
			t.Errorf("unmetSetting(%q, %q) = %v, want %v", tc.have, tc.want, got, tc.unmet)
		}
	}
}
