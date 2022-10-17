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

package injection

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
)

type resourceKey struct{}

// LoggingContextOrDie injects a logger into the returned context. The logger is
// configured by the ConfigMap `config-logging` and live updates the level.
func LoggingContextOrDie(componentName string, config *rest.Config, cmw *informer.InformedWatcher) context.Context {
	ctx, startinformers := injection.EnableInjectionOrDie(signals.NewContext(), config)
	logger, atomicLevel := sharedmain.SetupLoggerOrDie(ctx, componentName)
	ctx = logging.WithLogger(ctx, logger)
	rest.SetDefaultWarningHandler(&logging.WarningHandler{Logger: logger})
	sharedmain.WatchLoggingConfigOrDie(ctx, cmw, logger, atomicLevel, componentName)
	startinformers()
	return ctx
}

func WithNamespacedName(ctx context.Context, namespacedname types.NamespacedName) context.Context {
	return context.WithValue(ctx, resourceKey{}, namespacedname)
}

func GetNamespacedName(ctx context.Context) types.NamespacedName {
	retval := ctx.Value(resourceKey{})
	if retval == nil {
		return types.NamespacedName{}
	}
	return retval.(types.NamespacedName)
}

type controllerNameKeyType struct{}

var controllerNameKey = controllerNameKeyType{}

func WithControllerName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, controllerNameKey, name)
}

func GetControllerName(ctx context.Context) string {
	name := ctx.Value(controllerNameKey)
	if name == nil {
		return ""
	}
	return name.(string)
}
