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

package version

import (
	"context"
	"fmt"
	"strings"

	"github.com/patrickmn/go-cache"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/karpenter/pkg/utils/pretty"
)

const (
	kubernetesVersionCacheKey = "kubernetesVersion"
	// Karpenter's supported version of Kubernetes
	// If a user runs a karpenter image on a k8s version outside the min and max,
	// One error message will be fired to notify
	MinK8sVersion = "1.23"
	MaxK8sVersion = "1.30"
)

type Provider interface {
	Get(ctx context.Context) (string, error)
}

// DefaultProvider get the APIServer version. This will be initialized at start up and allows karpenter to have an understanding of the cluster version
// for decision making. The version is cached to help reduce the amount of calls made to the API Server
type DefaultProvider struct {
	cache               *cache.Cache
	cm                  *pretty.ChangeMonitor
	kubernetesInterface kubernetes.Interface
}

func NewDefaultProvider(kubernetesInterface kubernetes.Interface, cache *cache.Cache) *DefaultProvider {
	return &DefaultProvider{
		cm:                  pretty.NewChangeMonitor(),
		cache:               cache,
		kubernetesInterface: kubernetesInterface,
	}
}

func (p *DefaultProvider) Get(ctx context.Context) (string, error) {
	if version, ok := p.cache.Get(kubernetesVersionCacheKey); ok {
		return version.(string), nil
	}
	serverVersion, err := p.kubernetesInterface.Discovery().ServerVersion()
	if err != nil {
		return "", err
	}
	version := fmt.Sprintf("%s.%s", serverVersion.Major, strings.TrimSuffix(serverVersion.Minor, "+"))
	p.cache.SetDefault(kubernetesVersionCacheKey, version)
	if p.cm.HasChanged("kubernetes-version", version) {
		log.FromContext(ctx).WithValues("version", version).V(1).Info("discovered kubernetes version")
		if err := validateK8sVersion(version); err != nil {
			log.FromContext(ctx).Error(err, "failed validating kubernetes version")
		}
	}
	return version, nil
}

func validateK8sVersion(v string) error {
	k8sVersion := version.MustParseGeneric(v)

	// We will only error if the user is running karpenter on a k8s version,
	// that is out of the range of the minK8sVersion and maxK8sVersion
	if k8sVersion.LessThan(version.MustParseGeneric(MinK8sVersion)) ||
		version.MustParseGeneric(MaxK8sVersion).LessThan(k8sVersion) {
		return fmt.Errorf("karpenter version is not compatible with K8s version %s", k8sVersion)
	}

	return nil
}
