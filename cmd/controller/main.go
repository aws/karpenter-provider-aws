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

package main

import (
	"fmt"

	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/aws"
	"github.com/aws/karpenter/pkg/controllers"
	"github.com/aws/karpenter/pkg/startup"
)

func main() {
	options := startup.Initialize()
	cloudProvider := startup.Decorate(aws.NewCloudProvider(options.Ctx, cloudprovider.Options{
		ClientSet:  options.Clientset,
		KubeClient: options.Manager.GetClient(),
		StartAsync: options.Manager.Elected(),
	}), options.Manager)
	if err := startup.RegisterControllers(options.Ctx,
		options.Manager,
		controllers.GetControllers(options.Ctx, options.Clock, options.Cmw, options.Recorder, options.Manager.GetClient(), options.Clientset, cloudProvider)...,
	).Start(options.Ctx); err != nil {
		panic(fmt.Sprintf("Unable to start manager, %s", err))
	}
}
