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

	"github.com/aws/karpenter/pkg/utils/options"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
)

type resourceKey struct{}

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

type optionsKey struct{}

func WithOptions(ctx context.Context, opts options.Options) context.Context {
	return context.WithValue(ctx, optionsKey{}, opts)
}

func GetOptions(ctx context.Context) options.Options {
	retval := ctx.Value(optionsKey{})
	if retval == nil {
		return options.Options{}
	}
	return retval.(options.Options)
}

type configKey struct{}

func WithConfig(ctx context.Context, config *rest.Config) context.Context {
	return context.WithValue(ctx, configKey{}, config)
}

func GetConfig(ctx context.Context) *rest.Config {
	retval := ctx.Value(configKey{})
	if retval == nil {
		return nil
	}
	return retval.(*rest.Config)
}
