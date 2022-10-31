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

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/apis/config/settings"
	"github.com/aws/karpenter-core/pkg/utils/functional"
	awssettings "github.com/aws/karpenter/pkg/apis/config/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/test/pkg/environment/common"
)

var (
	//nolint:govet
	CleanableObjects = []functional.Pair[client.Object, client.ObjectList]{
		{&v1alpha1.AWSNodeTemplate{}, &v1alpha1.AWSNodeTemplateList{}},
	}
)

func (env *Environment) BeforeEach(opts ...common.Option) {
	options := common.ResolveOptions(opts)
	if options.EnableDebug {
		fmt.Println("------- START AWS BEFORE -------")
		defer fmt.Println("------- END AWS BEFORE -------")
	}
	env.ExpectSettingsCreatedOrUpdated(settings.Registration.DefaultData, awssettings.Registration.DefaultData)
	env.Environment.BeforeEach(opts...)
}

func (env *Environment) AfterEach(opts ...common.Option) {
	options := common.ResolveOptions(opts)
	if options.EnableDebug {
		fmt.Println("------- START AWS AFTER -------")
		defer fmt.Println("------- END AWS AFTER -------")
	}
	env.ExpectSettingsDeleted()
	env.Environment.CleanupObjects(CleanableObjects, options)
	env.Environment.AfterEach(opts...)
}
