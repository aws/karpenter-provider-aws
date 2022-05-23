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
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/events"
)

func Get(ctx context.Context, key interface{}) interface{} {
	value := ctx.Value(key)
	if value == nil {
		panic(fmt.Sprintf("nil value for key %s and context %s", reflect.TypeOf(key), ctx))
	}
	return value
}

type kubeClientKey struct{}

func InjectKubeClient(ctx context.Context, value client.Client) context.Context {
	return context.WithValue(ctx, kubeClientKey{}, value)
}

func GetKubeClient(ctx context.Context) client.Client {
	return Get(ctx, kubeClientKey{}).(client.Client)
}

type kubernetesInterfaceKey struct{}

func InjectKubernetesInterface(ctx context.Context, value kubernetes.Interface) context.Context {
	return context.WithValue(ctx, kubernetesInterfaceKey{}, value)
}

func GetKubernetesInterface(ctx context.Context) kubernetes.Interface {
	return Get(ctx, kubernetesInterfaceKey{}).(kubernetes.Interface)
}

type eventRecorderKey struct{}

func InjectEventRecorder(ctx context.Context, value events.Recorder) context.Context {
	return context.WithValue(ctx, eventRecorderKey{}, value)
}

func GetEventRecorder(ctx context.Context) events.Recorder {
	return Get(ctx, eventRecorderKey{}).(events.Recorder)
}

type namespacedNameKey struct{}

func InjectNamespacedName(ctx context.Context, value types.NamespacedName) context.Context {
	return context.WithValue(ctx, namespacedNameKey{}, value)
}

func GetNamespacedName(ctx context.Context) types.NamespacedName {
	return Get(ctx, namespacedNameKey{}).(types.NamespacedName)
}

type restConfigKey struct{}

func InjectRestConfig(ctx context.Context, value *rest.Config) context.Context {
	return context.WithValue(ctx, restConfigKey{}, value)
}

func GetRestConfig(ctx context.Context) *rest.Config {
	return Get(ctx, restConfigKey{}).(*rest.Config)
}

type controllerNameKey struct{}

func InjectControllerName(ctx context.Context, value string) context.Context {
	return context.WithValue(ctx, controllerNameKey{}, value)
}

func GetControllerName(ctx context.Context) string {
	return Get(ctx, controllerNameKey{}).(string)
}
