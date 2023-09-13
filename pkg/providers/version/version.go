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
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/patrickmn/go-cache"
	v1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/version"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/utils/pretty"
)

const (
	kubernetesVersionCacheKey = "kubernetesVersion"
)

type HTTPClient interface {
	Get(string) (*http.Response, error)
}

// Provider get the APIServer version. This will be initialized at start up and allows karpenter to have an understanding of the cluster version
// for decision making. The version is cached to help reduce the amount of calls made to the API Server
type Provider struct {
	cache      *cache.Cache
	cm         *pretty.ChangeMonitor
	kubeClient client.Client
	httpClient HTTPClient
}

func NewProvider(cache *cache.Cache, client client.Client, httpClient HTTPClient) *Provider {
	return &Provider{
		cm:         pretty.NewChangeMonitor(),
		cache:      cache,
		kubeClient: client,
		httpClient: httpClient,
	}
}

func (p *Provider) Get(ctx context.Context) (string, error) {
	if version, ok := p.cache.Get(kubernetesVersionCacheKey); ok {
		return version.(string), nil
	}
	version, err := p.GetMinKubernetesVersion(ctx)
	if err != nil {
		return "", err
	}
	p.cache.SetDefault(kubernetesVersionCacheKey, version)
	if p.cm.HasChanged("kubernetes-version", version) {
		logging.FromContext(ctx).With("version", version).Debugf("discovered kubernetes version")
	}
	return version, nil
}

// GetMinKubernetesVersion ensures that we query all known APIServers for the K8s version.
// This to handle any scenarios where there may be multiple APIServer reporting different
// K8s versions.
func (p *Provider) GetMinKubernetesVersion(ctx context.Context) (string, error) {
	var endpointSlice v1.EndpointSlice
	if err := p.kubeClient.Get(ctx, types.NamespacedName{Name: "kubernetes", Namespace: "default"}, &endpointSlice); err != nil {
		return "", fmt.Errorf("getting endpoints, %w", err)
	}
	// This error should never occur
	minVersion, err := version.ParseGeneric("v99.99.99")
	if err != nil {
		return "", fmt.Errorf("parsing version, %w", err)
	}
	for _, address := range getAddresses(endpointSlice) {
		resp, err := p.httpClient.Get(address)
		if err != nil {
			return "", fmt.Errorf("sending get request, %w", err)
		}
		defer resp.Body.Close()
		var data map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return "", fmt.Errorf("reading response body, %w", err)
		}
		v, err := version.ParseGeneric(data["gitVersion"])
		if err != nil {
			return "", fmt.Errorf("parsing kubernetes version, %w", err)
		}
		if v.LessThan(minVersion) {
			minVersion = v
		}
	}
	// Return only the first two version numbers
	return strings.Join(strings.Split(minVersion.String(), ".")[0:2], "."), nil
}
func getAddresses(endpoints v1.EndpointSlice) []string {
	ports := []string{}
	// If there are no ports, it's the same as defining all ports.
	if len(endpoints.Ports) == 0 {
		ports = []string{""}
	} else {
		for _, port := range endpoints.Ports {
			ports = append(ports, fmt.Sprintf(":%s", strconv.Itoa(int(*port.Port))))
		}
	}
	ret := []string{}
	for _, endpoint := range endpoints.Endpoints {
		for _, address := range endpoint.Addresses {
			for _, port := range ports {
				ret = append(ret, fmt.Sprintf("https://%s%s/version", address, port))
			}
		}
	}
	return ret
}
