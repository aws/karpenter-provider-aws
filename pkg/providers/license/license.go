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

package license

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go/service/licensemanager"
	"github.com/aws/aws-sdk-go/service/licensemanager/licensemanageriface"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"

	"github.com/aws/karpenter/pkg/apis/v1beta1"

	"github.com/aws/karpenter-core/pkg/utils/pretty"
	"github.com/samber/lo"
	"knative.dev/pkg/logging"
)

type Provider struct {
	sync.RWMutex
	licensemanager licensemanageriface.LicenseManagerAPI
	cache          *cache.Cache
	cm             *pretty.ChangeMonitor
}

func NewProvider(lmapi licensemanageriface.LicenseManagerAPI, cache *cache.Cache) *Provider {
	return &Provider{
		licensemanager: lmapi,
		cm:             pretty.NewChangeMonitor(),
		// TODO: Remove cache for v1beta1, utilize resolved subnet from the AWSNodeTemplate.status
		// Subnets are sorted on AvailableIpAddressCount, descending order
		cache: cache,
	}
}

func (p *Provider) Get(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) ([]string, error) {
	p.Lock()
	defer p.Unlock()

	// Get selectors from the nodeClass, exit if no selectors defined
	selectors := nodeClass.Spec.LicenseSelectorTerms
	if selectors == nil {
		return nil, nil
	}

	// Look for a cached result
	hash, err := hashstructure.Hash(selectors, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	if err != nil {
		return nil, err
	}
	if cached, ok := p.cache.Get(fmt.Sprint(hash)); ok {
		return cached.([]string), nil
	}

	var licenses []string
	// Look up all License Configurations
	output, err := p.licensemanager.ListLicenseConfigurationsWithContext(ctx, &licensemanager.ListLicenseConfigurationsInput{})
	if err != nil {
		logging.FromContext(ctx).Errorf("listing license configurations %w", err)
		return nil, err
	}
	for i := range output.LicenseConfigurations {
		// filter results to only include those that match at least 1 selector
		for x := range selectors {
			if *output.LicenseConfigurations[i].Name == selectors[x].Name {
				licenses = append(licenses, *output.LicenseConfigurations[i].LicenseConfigurationArn)
			}
		}
	}
	p.cache.SetDefault(fmt.Sprint(hash), licenses)

	if p.cm.HasChanged(fmt.Sprintf("license/%t/%s", nodeClass.IsNodeTemplate, nodeClass.Name), licenses) {
		logging.FromContext(ctx).
			With("licenseProvider", licenses).
			Debugf("discovered license configuration")
	}
	return licenses, nil
}
