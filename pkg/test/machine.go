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

package test

import (
	"context"
	"fmt"
	"strings"

	"github.com/Pallinder/go-randomdata"
	"github.com/imdario/mergo"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
)

// Machine creates a test pod with defaults that can be overridden by MachineOptions.
// Overrides are applied in order, with a last write wins semantic.
func Machine(overrides ...v1alpha5.Machine) *v1alpha5.Machine {
	machine := &v1alpha5.Machine{}
	for _, opts := range overrides {
		if err := mergo.Merge(&machine, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge pod options: %s", err))
		}
	}
	if machine.Name == "" {
		machine.Name = strings.ToLower(randomdata.SillyName())
	}
	if machine.Spec.Provider == nil {
		machine.Spec.Provider = &runtime.RawExtension{}
	}
	machine.SetDefaults(context.Background())
	_ = machine.Validate(context.Background())
	return machine
}
