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

package fake

import (
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/licensemanager"
	"github.com/aws/aws-sdk-go/service/licensemanager/licensemanageriface"
)

type LicenseManagerAPI struct {
	licensemanageriface.LicenseManagerAPI
	LicenseManagerBehaviour
}
type LicenseManagerBehaviour struct {
	NextError                       AtomicError
	ListLicenseConfigurationsOutput AtomicPtr[licensemanager.ListLicenseConfigurationsOutput]
}

func (l *LicenseManagerAPI) Reset() {
	l.NextError.Reset()
	l.ListLicenseConfigurationsOutput.Reset()
}

func (l *LicenseManagerAPI) ListLicenseConfigurationsWithContext(_ aws.Context, _ *licensemanager.ListLicenseConfigurationsInput, _ ...request.Option) (*licensemanager.ListLicenseConfigurationsOutput, error) {
	if !l.NextError.IsNil() {
		return nil, l.NextError.Get()
	}
	if !l.ListLicenseConfigurationsOutput.IsNil() {
        return l.ListLicenseConfigurationsOutput.Clone(), nil
	}
	// fail if the test doesn't provide specific data which causes our pricing provider to use its static price list
	return nil, errors.New("no license data provided")
}
