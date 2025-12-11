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
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/awslabs/operatorpkg/serrors"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/aws-sdk-go-v2/service/eks"

	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"

	"github.com/aws/karpenter-provider-aws/pkg/operator/options"

	"sigs.k8s.io/karpenter/pkg/utils/pretty"
)

const (
	// Karpenter's supported version of Kubernetes
	// If a user runs a karpenter image on a k8s version outside the min and max,
	// One error message will be fired to notify
	MinK8sVersion = "1.26"
	MaxK8sVersion = "1.34"
)

type Provider interface {
	Get(ctx context.Context) string
}

// DefaultProvider get the APIServer version. This will be initialized at start up and allows karpenter to have an understanding of the cluster version
// for decision making. The version is cached to help reduce the amount of calls made to the API Server
type DefaultProvider struct {
	cm                  *pretty.ChangeMonitor
	kubernetesInterface kubernetes.Interface
	eksapi              sdk.EKSAPI
	version             atomic.Pointer[string]
}

func NewDefaultProvider(kubernetesInterface kubernetes.Interface, eksapi sdk.EKSAPI) *DefaultProvider {
	return &DefaultProvider{
		cm:                  pretty.NewChangeMonitor(),
		kubernetesInterface: kubernetesInterface,
		eksapi:              eksapi,
	}
}

func (p *DefaultProvider) Get(ctx context.Context) string {
	return *p.version.Load()
}

func (p *DefaultProvider) UpdateVersion(ctx context.Context) error {
	var version string
	var err error

	if options.FromContext(ctx).EKSControlPlane {
		version, err = p.getEKSVersion(ctx)
		if err != nil {
			return fmt.Errorf("validating kubernetes version, %w", err)
		}
	} else {
		version, err = p.getK8sVersion()
		if err != nil {
			return fmt.Errorf("validating kubernetes version, %w", err)
		}
	}
	p.version.Store(&version)
	return nil
}
func (p *DefaultProvider) UpdateVersionWithValidation(ctx context.Context) error {
	err := p.UpdateVersion(ctx)
	if err != nil {
		return err
	}
	version := p.Get(ctx)
	if p.cm.HasChanged("kubernetes-version", version) {
		log.FromContext(ctx).WithValues("version", version).V(1).Info("discovered kubernetes version")
		if err := validateK8sVersion(version); err != nil {
			return fmt.Errorf("validating kubernetes version, %w", err)
		}
	}
	return nil
}

// SupportedK8sVersions returns a slice of version strings in format "major.minor" for all versions of k8s supported by
// this version of Karpenter.
// Note: Assumes k8s only has a single major version (1.x)
func SupportedK8sVersions() []string {
	minMinor := lo.Must(strconv.Atoi(strings.Split(MinK8sVersion, ".")[1]))
	maxMinor := lo.Must(strconv.Atoi(strings.Split(MaxK8sVersion, ".")[1]))
	versions := make([]string, 0, maxMinor-minMinor+1)
	for i := minMinor; i <= maxMinor; i++ {
		versions = append(versions, fmt.Sprintf("1.%d", i))
	}
	return versions
}

func validateK8sVersion(v string) error {
	k8sVersion := version.MustParseGeneric(v)

	// We will only error if the user is running karpenter on a k8s version,
	// that is out of the range of the minK8sVersion and maxK8sVersion
	if k8sVersion.LessThan(version.MustParseGeneric(MinK8sVersion)) ||
		version.MustParseGeneric(MaxK8sVersion).LessThan(k8sVersion) {
		return serrors.Wrap(fmt.Errorf("karpenter is not compatible with kubernetes version"), "version", k8sVersion)
	}

	return nil
}

func (p *DefaultProvider) getEKSVersion(ctx context.Context) (string, error) {
	output, err := p.eksapi.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: lo.ToPtr(options.FromContext(ctx).ClusterName),
	})
	if err != nil {
		return "", err
	}
	return lo.FromPtr(output.Cluster.Version), nil
}

func (p *DefaultProvider) getK8sVersion() (string, error) {
	output, err := p.kubernetesInterface.Discovery().ServerVersion()
	if err != nil || output == nil {
		return "", fmt.Errorf("getting kubernetes version from the kubernetes API")
	}
	return fmt.Sprintf("%s.%s", output.Major, strings.TrimSuffix(output.Minor, "+")), err
}
