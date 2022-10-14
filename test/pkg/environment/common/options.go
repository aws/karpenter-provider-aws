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

package common

import "sigs.k8s.io/controller-runtime/pkg/client"

type Option func(Options) Options

type Options struct {
	EnableDebug      bool
	IgnoreCleanupFor []client.Object
}

func EnableDebug(o Options) Options {
	o.EnableDebug = true
	return o
}

func IgnoreCleanupFor(objs ...client.Object) func(Options) Options {
	return func(o Options) Options {
		o.IgnoreCleanupFor = append(o.IgnoreCleanupFor, objs...)
		return o
	}
}

func ResolveOptions(opts []Option) Options {
	o := Options{}
	for _, opt := range opts {
		if opt != nil {
			o = opt(o)
		}
	}
	return o
}
