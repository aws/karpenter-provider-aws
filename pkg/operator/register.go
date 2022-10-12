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

package operator

import (
	"context"
	"fmt"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// RegisterControllers registers a set of controllers to the controller manager
func RegisterControllers(ctx context.Context, m manager.Manager, controllers ...Controller) manager.Manager {
	for _, c := range controllers {
		if err := c.Register(ctx, m); err != nil {
			panic(err)
		}
		// if the controller implements a liveness check, connect it
		if lp, ok := c.(HealthCheck); ok {
			utilruntime.Must(m.AddHealthzCheck(fmt.Sprintf("%T", c), lp.LivenessProbe))
		}
	}
	if err := m.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		panic(fmt.Sprintf("Failed to add health probe, %s", err))
	}
	if err := m.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		panic(fmt.Sprintf("Failed to add ready probe, %s", err))
	}
	return m
}
