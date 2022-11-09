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
	"fmt"

	//nolint:revive,stylecheck
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/utils/functional"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/test/pkg/environment/common"
)

var persistedSettings *v1.ConfigMap

var (
	CleanableObjects = []functional.Pair[client.Object, client.ObjectList]{
		{First: &v1alpha1.AWSNodeTemplate{}, Second: &v1alpha1.AWSNodeTemplateList{}},
	}
)

func (env *Environment) BeforeEach(opts ...common.Option) {
	persistedSettings = env.ExpectSettings()
	env.Environment.BeforeEach(opts...)
}

func (env *Environment) Cleanup(opts ...common.Option) {
	options := common.ResolveOptions(opts)
	if !options.DisableDebug {
		fmt.Println("------- START AWS CLEANUP -------")
		defer fmt.Println("------- END AWS CLEANUP -------")
	}
	env.ExpectCreatedOrUpdated(persistedSettings)
	env.Environment.CleanupObjects(CleanableObjects)
	env.Environment.Cleanup(opts...)
}

func (env *Environment) ForceCleanup(opts ...common.Option) {
	env.Environment.ForceCleanup(opts...)
}

func (env *Environment) AfterEach(opts ...common.Option) {
	env.Environment.AfterEach(opts...)
}
