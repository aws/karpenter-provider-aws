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
	"os"
	"strconv"
	"strings"

	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	v1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"

	"github.com/aws/karpenter-core/pkg/utils/pretty"
	"github.com/aws/karpenter/pkg/apis/settings"
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
	cache               *cache.Cache
	cm                  *pretty.ChangeMonitor
	kubeClient          client.Client
	httpClient          HTTPClient
	kubernetesInterface kubernetes.Interface
	eks                 eksiface.EKSAPI
}

func NewProvider(kubernetesInterface kubernetes.Interface, cache *cache.Cache, client client.Client,
	httpClient HTTPClient, eks eksiface.EKSAPI) *Provider {
	return &Provider{
		cm:                  pretty.NewChangeMonitor(),
		cache:               cache,
		kubeClient:          client,
		httpClient:          httpClient,
		kubernetesInterface: kubernetesInterface,
		eks:                 eks,
	}
}

func (p *Provider) Get(ctx context.Context) (string, error) {
	if version, ok := p.cache.Get(kubernetesVersionCacheKey); ok {
		return version.(string), nil
	}
	version, err := p.getKubernetesVersion(ctx)
	if err != nil {
		return "", fmt.Errorf("getting kubernetes version, %w", err)
	}
	p.cache.SetDefault(kubernetesVersionCacheKey, version)
	if p.cm.HasChanged("kubernetes-version", version) {
		logging.FromContext(ctx).With("version", version).Debugf("discovered kubernetes version")
	}
	return version, nil
}

// getKubernetesVersion will try to get the Kubernetes version from three sources of truth.
// This is done since it's possible for the APIServers to disagree on K8s version.
// 1. If Karpenter is running inside the cluster, it'll ping the private APIServer IPs.
// 2. If Karpneter is running in EKS, it'll call the EKS.DescribeCluster API
// 3. Fallback to the Kubernetes clientset to get a kubernetes version.
func (p *Provider) getKubernetesVersion(ctx context.Context) (string, error) {
	var version string
	var err error
	// If we're running locally, these environment variables will be empty.
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if len(host) != 0 && len(port) != 0 {
		version, err = p.getMinKubernetesVersion(ctx)
		if err != nil {
			return "", err
		}
		return version, nil
	}
	version, err = p.getClusterVersion(ctx)
	if err == nil {
		return version, nil
	}
	logging.FromContext(ctx).Infof("failed to get kubernetes version from EKS cluster %w", err)
	serverVersion, err := p.kubernetesInterface.Discovery().ServerVersion()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s", serverVersion.Major, strings.TrimSuffix(serverVersion.Minor, "+")), nil
}

// GetMinKubernetesVersion ensures that we query all known APIServers for the K8s version.
// This to handle any scenarios where there may be multiple APIServer reporting different
// K8s versions.
func (p *Provider) getMinKubernetesVersion(ctx context.Context) (string, error) {
	var endpointSlice v1.EndpointSlice
	if err := p.kubeClient.Get(ctx, types.NamespacedName{Name: "kubernetes", Namespace: "default"}, &endpointSlice); err != nil {
		return "", fmt.Errorf("getting endpoints, %w", err)
	}
	var minVersion *version.Version
	for _, address := range getAddresses(endpointSlice) {
		if err := func() error {
			resp, err := p.httpClient.Get(address)
			if err != nil {
				return fmt.Errorf("sending get request, %w", err)
			}
			// Close the body to avoid leaking file descriptors
			// Always read the body so we can re-use the connection:
			// https://stackoverflow.com/questions/17948827/reusing-http-connections-in-go
			defer resp.Body.Close()
			var data map[string]string
			if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
				return fmt.Errorf("reading response body, %w", err)
			}
			v, err := version.ParseGeneric(data["gitVersion"])
			if err != nil {
				return fmt.Errorf("parsing kubernetes version, %w", err)
			}
			if minVersion == nil || v.LessThan(minVersion) {
				minVersion = v
			}
			return nil
		}(); err != nil {
			return "", err
		}
	}
	if minVersion == nil {
		return "", fmt.Errorf("failed to get a kubernetes version")
	}
	// Only return the major and minor versions.
	return strings.Join(strings.Split(minVersion.String(), ".")[0:2], "."), nil
}

func (p *Provider) getClusterVersion(ctx context.Context) (string, error) {
	out, err := p.eks.DescribeCluster(&eks.DescribeClusterInput{
		Name: aws.String(settings.FromContext(ctx).ClusterName),
	})
	if err != nil {
		return "", fmt.Errorf("describing cluster, %w", err)
	}
	if out == nil || out.Cluster == nil || out.Cluster.Version == nil {
		return "", fmt.Errorf("failed to retrieve an eks cluster version")
	}
	return *out.Cluster.Version, nil
}

func getAddresses(endpoints v1.EndpointSlice) []string {
	// If there are no ports, it's the same as defining all ports.
	ports := []string{""}
	if len(endpoints.Ports) > 0 {
		ports = lo.Map(endpoints.Ports, func(p v1.EndpointPort, _ int) string { return fmt.Sprintf(":%s", strconv.Itoa(int(*p.Port))) })
	}
	var ret []string
	for _, endpoint := range endpoints.Endpoints {
		for _, address := range endpoint.Addresses {
			for _, port := range ports {
				ret = append(ret, fmt.Sprintf("https://%s%s/version", address, port))
			}
		}
	}
	return ret
}
