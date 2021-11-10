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

package options

import (
	"context"
	"fmt"
	"net/url"

	"go.uber.org/multierr"
)

// Options for running this binary
type Options struct {
	ClusterName     string
	ClusterEndpoint string
	MetricsPort     int
	HealthProbePort int
	KubeClientQPS   int
	KubeClientBurst int
}

type optionsKey struct{}

func Get(ctx context.Context) Options {
	return ctx.Value(optionsKey{}).(Options)
}

func Inject(ctx context.Context, options Options) context.Context {
	return context.WithValue(ctx, optionsKey{}, options)
}

func (o Options) Validate() (err error) {
	err = multierr.Append(err, o.validateEndpoint())
	if o.ClusterName == "" {
		err = multierr.Append(err, fmt.Errorf("CLUSTER_NAME is required"))
	}
	return err
}

func (o Options) validateEndpoint() error {
	endpoint, err := url.Parse(o.ClusterEndpoint)
	// url.Parse() will accept a lot of input without error; make
	// sure it's a real URL
	if err != nil || !endpoint.IsAbs() || endpoint.Hostname() == "" {
		return fmt.Errorf("\"%s\" not a valid CLUSTER_ENDPOINT URL", o.ClusterEndpoint)
	}
	return nil
}
