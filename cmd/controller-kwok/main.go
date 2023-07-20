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
	"log"
	"net/http"
	_ "net/http/pprof"

	corecontrollers "github.com/aws/karpenter-core/pkg/controllers"
	"github.com/aws/karpenter-core/pkg/controllers/state"
	coreoperator "github.com/aws/karpenter-core/pkg/operator"
	"github.com/aws/karpenter/cmd/controller-kwok/kwok"
)

func main() {
	ctx, op := coreoperator.NewOperator()

	log.Println("starting karpenter-kwok")
	go func() {
		debugPort := 6060
		log.Printf("debug port is listening on %d", debugPort)
		log.Println(http.ListenAndServe(fmt.Sprintf(":%d", debugPort), nil))
	}()

	cloudProvider := kwok.NewCloudProvider(op.KubernetesInterface)
	op.
		WithControllers(ctx, corecontrollers.NewControllers(
			ctx,
			op.Clock,
			op.GetClient(),
			op.KubernetesInterface,
			state.NewCluster(op.Clock, op.GetClient(), cloudProvider),
			op.EventRecorder,
			cloudProvider,
		)...).Start(ctx)
}
