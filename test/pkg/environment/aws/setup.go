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

package aws

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

var persistedSettings []corev1.EnvVar

var (
	CleanableObjects = []client.Object{
		&v1.EC2NodeClass{},
	}
)

func (env *Environment) BeforeEach() {
	persistedSettings = env.ExpectSettings()
	env.Environment.BeforeEach()
}

func (env *Environment) Cleanup() {
	env.Environment.Cleanup()
	env.CleanupObjects(CleanableObjects...)
}

func (env *Environment) AfterEach() {
	env.Environment.AfterEach()
	// Ensure we reset settings after collecting the controller logs
	env.ExpectSettingsReplaced(persistedSettings...)
}
