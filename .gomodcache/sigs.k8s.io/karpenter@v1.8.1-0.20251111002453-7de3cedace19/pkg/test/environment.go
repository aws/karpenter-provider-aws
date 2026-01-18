/*
Copyright The Kubernetes Authors.

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

package test

import (
	"context"
	"log"
	"strings"

	"github.com/awslabs/operatorpkg/option"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/utils/env"
)

type Environment struct {
	envtest.Environment

	Client              client.Client
	KubernetesInterface kubernetes.Interface
	Version             *version.Version
	Done                chan struct{}
	Cancel              context.CancelFunc
}

type EnvironmentOptions struct {
	crds          []*apiextensionsv1.CustomResourceDefinition
	fieldIndexers []func(cache.Cache) error
	configOptions []func(*rest.Config)
}

// WithCRDs registers the specified CRDs to the apiserver for use in testing
func WithCRDs(crds ...*apiextensionsv1.CustomResourceDefinition) option.Function[EnvironmentOptions] {
	return func(o *EnvironmentOptions) {
		o.crds = append(o.crds, crds...)
	}
}

// WithFieldIndexers expects a function that indexes fields against the cache such as cache.IndexField(...).
//
// Note: Only use when necessary, the use of field indexers in functional tests requires the use of the cache syncing
// client, which can have significant drawbacks for test performance.
func WithFieldIndexers(fieldIndexers ...func(cache.Cache) error) option.Function[EnvironmentOptions] {
	return func(o *EnvironmentOptions) {
		o.fieldIndexers = append(o.fieldIndexers, fieldIndexers...)
	}
}

// WithConfigOptions allows customization of the rest.Config before client creation
func WithConfigOptions(options ...func(*rest.Config)) option.Function[EnvironmentOptions] {
	return func(o *EnvironmentOptions) {
		o.configOptions = append(o.configOptions, options...)
	}
}

func NodeProviderIDFieldIndexer(ctx context.Context) func(cache.Cache) error {
	return func(c cache.Cache) error {
		return c.IndexField(ctx, &corev1.Node{}, "spec.providerID", func(obj client.Object) []string {
			return []string{obj.(*corev1.Node).Spec.ProviderID}
		})
	}
}

func NodeClaimProviderIDFieldIndexer(ctx context.Context) func(cache.Cache) error {
	return func(c cache.Cache) error {
		return c.IndexField(ctx, &v1.NodeClaim{}, "status.providerID", func(obj client.Object) []string {
			return []string{obj.(*v1.NodeClaim).Status.ProviderID}
		})
	}
}

func NodeClaimNodeClassRefFieldIndexer(ctx context.Context) func(cache.Cache) error {
	return func(c cache.Cache) error {
		var err error
		err = multierr.Append(err, c.IndexField(ctx, &v1.NodeClaim{}, "spec.nodeClassRef.group", func(obj client.Object) []string {
			return []string{obj.(*v1.NodeClaim).Spec.NodeClassRef.Group}
		}))
		err = multierr.Append(err, c.IndexField(ctx, &v1.NodeClaim{}, "spec.nodeClassRef.kind", func(obj client.Object) []string {
			return []string{obj.(*v1.NodeClaim).Spec.NodeClassRef.Kind}
		}))
		err = multierr.Append(err, c.IndexField(ctx, &v1.NodeClaim{}, "spec.nodeClassRef.name", func(obj client.Object) []string {
			return []string{obj.(*v1.NodeClaim).Spec.NodeClassRef.Name}
		}))
		return err
	}
}

func NodePoolNodeClassRefFieldIndexer(ctx context.Context) func(cache.Cache) error {
	return func(c cache.Cache) error {
		var err error
		err = multierr.Append(err, c.IndexField(ctx, &v1.NodePool{}, "spec.template.spec.nodeClassRef.group", func(obj client.Object) []string {
			return []string{obj.(*v1.NodePool).Spec.Template.Spec.NodeClassRef.Group}
		}))
		err = multierr.Append(err, c.IndexField(ctx, &v1.NodePool{}, "spec.template.spec.nodeClassRef.kind", func(obj client.Object) []string {
			return []string{obj.(*v1.NodePool).Spec.Template.Spec.NodeClassRef.Kind}
		}))
		err = multierr.Append(err, c.IndexField(ctx, &v1.NodePool{}, "spec.template.spec.nodeClassRef.name", func(obj client.Object) []string {
			return []string{obj.(*v1.NodePool).Spec.Template.Spec.NodeClassRef.Name}
		}))
		return err
	}
}

func VolumeAttachmentFieldIndexer(ctx context.Context) func(cache.Cache) error {
	return func(c cache.Cache) error {
		return c.IndexField(ctx, &storagev1.VolumeAttachment{}, "spec.nodeName", func(obj client.Object) []string {
			return []string{obj.(*storagev1.VolumeAttachment).Spec.NodeName}
		})
	}
}

func NewEnvironment(options ...option.Function[EnvironmentOptions]) *Environment {
	opts := option.Resolve(options...)
	ctx, cancel := context.WithCancel(context.Background())

	version := version.MustParseSemantic(strings.ReplaceAll(env.WithDefaultString("K8S_VERSION", "1.34.x"), ".x", ".0"))
	environment := envtest.Environment{Scheme: scheme.Scheme, CRDs: opts.crds}
	if version.Minor() >= 21 && version.Minor() < 32 {
		// PodAffinityNamespaceSelector is used for label selectors in pod affinities.  If the feature-gate is turned off,
		// the api-server just clears out the label selector so we never see it.  If we turn it on, the label selectors
		// are passed to us and we handle them. This feature is alpha in apiextensionsv1.21, beta in apiextensionsv1.22 and will be GA in 1.24. See
		// https://github.com/kubernetes/enhancements/issues/2249 for more info.
		environment.ControlPlane.GetAPIServer().Configure().Set("feature-gates", "PodAffinityNamespaceSelector=true")
	}
	//MinDomains got promoted to stable in 1.32
	if version.Minor() >= 24 && version.Minor() < 32 {
		// MinDomainsInPodTopologySpread enforces a minimum number of eligible node domains for pod scheduling
		// See https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/#spread-constraint-definition
		// Ref: https://github.com/aws/karpenter-core/pull/330
		environment.ControlPlane.GetAPIServer().Configure().Set("feature-gates", "MinDomainsInPodTopologySpread=true")
	}

	_ = lo.Must(environment.Start())

	// Apply any config overrides
	for _, option := range opts.configOptions {
		option(environment.Config)
	}

	// We use a modified client if we need field indexers
	var c client.Client
	if len(opts.fieldIndexers) > 0 {
		cache := lo.Must(cache.New(environment.Config, cache.Options{Scheme: scheme.Scheme}))
		for _, index := range opts.fieldIndexers {
			lo.Must0(index(cache))
		}
		lo.Must0(cache.IndexField(ctx, &corev1.Pod{}, "spec.nodeName", func(o client.Object) []string {
			pod := o.(*corev1.Pod)
			return []string{pod.Spec.NodeName}
		}))
		c = &CacheSyncingClient{
			Client: lo.Must(client.New(environment.Config, client.Options{Scheme: scheme.Scheme, Cache: &client.CacheOptions{Reader: cache}})),
		}
		go func() {
			lo.Must0(cache.Start(ctx))
		}()
		if !cache.WaitForCacheSync(ctx) {
			log.Fatalf("cache failed to sync")
		}
	} else {
		c = lo.Must(client.New(environment.Config, client.Options{Scheme: scheme.Scheme}))
	}
	return &Environment{
		Environment:         environment,
		Client:              c,
		KubernetesInterface: kubernetes.NewForConfigOrDie(environment.Config),
		Version:             version,
		Done:                make(chan struct{}),
		Cancel:              cancel,
	}
}

func (e *Environment) Stop() error {
	close(e.Done)
	e.Cancel()
	return e.Environment.Stop()
}
