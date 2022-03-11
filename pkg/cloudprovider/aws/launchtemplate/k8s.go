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

package launchtemplate

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/patrickmn/go-cache"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/logging"
)

const kubernetesVersionCacheKey = "kubernetesVersion"

type K8sVersion struct {
	ServerVersion version.Info
	Major         uint64
	Minor         uint64
}

func NewK8sVersion(serverVersion version.Info) (K8sVersion, error) {
	var v K8sVersion
	major, err := strconv.ParseUint(serverVersion.Major, 10, 64)
	if err != nil {
		return v, fmt.Errorf("invalid major version %s, %w", serverVersion.Major, err)
	}
	minor, err := strconv.ParseUint(strings.TrimSuffix(serverVersion.Minor, "+"), 10, 64)
	if err != nil {
		return v, fmt.Errorf("invalid minor version %s, %w", serverVersion.Minor, err)
	}
	return K8sVersion{
		ServerVersion: serverVersion,
		Major:         major,
		Minor:         minor,
	}, nil
}

func (v *K8sVersion) String() string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

type K8sClient interface {
	ServerVersion(ctx context.Context) (*K8sVersion, error)
}

type NativeK8sClient struct {
	clientSet *kubernetes.Clientset
}

func NewNativeK8sClient(clientSet *kubernetes.Clientset) *NativeK8sClient {
	return &NativeK8sClient{
		clientSet: clientSet,
	}
}

func (b *NativeK8sClient) ServerVersion(ctx context.Context) (*K8sVersion, error) {
	serverVersion, err := b.clientSet.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}
	version, err := NewK8sVersion(*serverVersion)
	if err != nil {
		return nil, err
	}
	return &version, nil
}

type CachingK8sClient struct {
	cache *cache.Cache
	inner K8sClient
}

func NewCachingK8sClient(inner K8sClient) *CachingK8sClient {
	return &CachingK8sClient{
		inner: inner,
		cache: cache.New(CacheTTL, CacheCleanupInterval),
	}
}

func (b *CachingK8sClient) ServerVersion(ctx context.Context) (*K8sVersion, error) {
	if version, ok := b.cache.Get(kubernetesVersionCacheKey); ok {
		return version.(*K8sVersion), nil
	}
	version, err := b.inner.ServerVersion(ctx)
	if err != nil {
		return nil, err
	}
	b.cache.SetDefault(kubernetesVersionCacheKey, &version)
	logging.FromContext(ctx).Debugf("Discovered kubernetes version %s", version)
	return version, nil
}
