# ComponentConfig Controller Runtime Support
Author: @christopherhein

Last Updated on: 03/02/2020

## Table of Contents

<!--ts-->
   * [ComponentConfig Controller Runtime Support](#componentconfig-controller-runtime-support)
      * [Table of Contents](#table-of-contents)
      * [Summary](#summary)
      * [Motivation](#motivation)
         * [Links to Open Issues](#links-to-open-issues)
         * [Goals](#goals)
         * [Non-Goals/Future Work](#non-goalsfuture-work)
      * [Proposal](#proposal)
		* [ComponentConfig Load Order](#componentconfig-load-order)
		* [Embeddable ComponentConfig Type](#embeddable-componentconfig-type)
		* [Default ComponentConfig Type](#default-componentconfig-type)
		* [Using Flags w/ ComponentConfig](#using-flags-w-componentconfig)
		* [Kubebuilder Scaffolding Example](#kubebuilder-scaffolding-example)
      * [User Stories](#user-stories)
         * [Controller Author with controller-runtime and default type](#controller-author-with-controller-runtime-and-default-type)
         * [Controller Author with controller-runtime and custom type](#controller-author-with-controller-runtime-and-custom-type)
         * [Controller Author with kubebuilder (tbd proposal for kubebuilder)](#controller-author-with-kubebuilder-tbd-proposal-for-kubebuilder)
         * [Controller User without modifications to config](#controller-user-without-modifications-to-config)
         * [Controller User with modifications to config](#controller-user-with-modifications-to-config)
      * [Risks and Mitigations](#risks-and-mitigations)
      * [Alternatives](#alternatives)
      * [Implementation History](#implementation-history)

<!--te-->

## Summary

Currently controllers that use `controller-runtime` need to configure the `ctrl.Manager` by using flags or hardcoding values into the initialization methods. Core Kubernetes has started to move away from using flags as a mechanism for configuring components and standardized on [`ComponentConfig` or Versioned Component Configuration Files](https://docs.google.com/document/d/1FdaEJUEh091qf5B98HM6_8MS764iXrxxigNIdwHYW9c/edit). This proposal is to bring `ComponentConfig` to `controller-runtime` to allow controller authors to make `go` types backed by `apimachinery` to unmarshal and configure the `ctrl.Manager{}` reducing the flags and allowing code based tools to easily configure controllers instead of requiring them to mutate CLI args.

## Motivation

This change is important because:
- it will help make it easier for controllers to be configured by other machine processes
- it will reduce the required flags required to start a controller
- allow for configuration types which aren't natively supported by flags
- allow using and upgrading older configurations avoiding breaking changes in flags

### Links to Open Issues

- [#518 Provide a ComponentConfig to tweak the Manager](https://github.com/kubernetes-sigs/controller-runtime/issues/518)
- [#207 Reduce command line flag boilerplate](https://github.com/kubernetes-sigs/controller-runtime/issues/207)
- [#722 Implement ComponentConfig by default & stop using (most) flags](https://github.com/kubernetes-sigs/kubebuilder/issues/722)

### Goals

- Provide an interface for pulling configuration data out of exposed `ComponentConfig` types (see below for implementation)
- Provide a new `ctrl.NewFromComponentConfig()` function for initializing a manager
- Provide an embeddable `ControllerManagerConfiguration` type for easily authoring `ComponentConfig` types
- Provide an `DefaultControllerConfig` to make the switching easier for clients

### Non-Goals/Future Work

- `kubebuilder` implementation and design in another PR
- Changing the default `controller-runtime` implementation
- Dynamically reloading `ComponentConfig` object
- Providing `flags` interface and overrides

## Proposal

The `ctrl.Manager` _SHOULD_ support loading configurations from `ComponentConfig` like objects.
An interface for that object with getters for the specific configuration parameters is created to bridge existing patterns.

Without breaking the current `ctrl.NewManager` which uses an exported `ctrl.Options{}` the `manager.go` can expose a new func, `NewFromComponentConfig()` this would be able to loop through the getters to populate an internal `ctrl.Options{}` and pass that into `New()`.

```golang
//pkg/manager/manager.go

// ManagerConfiguration defines what the ComponentConfig object for ControllerRuntime needs to support
type ManagerConfiguration interface {
	GetSyncPeriod() *time.Duration

	GetLeaderElection() bool
	GetLeaderElectionNamespace() string
	GetLeaderElectionID() string

	GetLeaseDuration() *time.Duration
	GetRenewDeadline() *time.Duration
	GetRetryPeriod() *time.Duration

	GetNamespace() string
	GetMetricsBindAddress() string
	GetHealthProbeBindAddress() string

	GetReadinessEndpointName() string
	GetLivenessEndpointName() string

	GetPort() int
	GetHost() string

	GetCertDir() string
}

func NewFromComponentConfig(config *rest.Config, scheme *runtime.Scheme, filename string, managerconfig ManagerConfiguration) (Manager, error) {
	codecs := serializer.NewCodecFactory(scheme)
    if err := decodeComponentConfigFileInto(codecs, filename, managerconfig); err != nil {

	}
	options := Options{}

	if scheme != nil {
		options.Scheme = scheme
	}

	// Loop through getters
	if managerconfig.GetLeaderElection() {
		options.LeaderElection = managerconfig.GetLeaderElection()
	}
	// ...

	return New(config, options)
}
```

#### ComponentConfig Load Order

![ComponentConfig Load Order](/designs/images/component-config-load.png)

#### Embeddable ComponentConfig Type

To make this easier for Controller authors `controller-runtime` can expose a set of `config.ControllerConfiguration` type that can be embedded similar to the way that `k8s.io/apimachinery/pkg/apis/meta/v1` works for `TypeMeta` and `ObjectMeta` these could live in `pkg/api/config/v1alpha1/types.go`. See the `DefaultComponentConfig` type below for and example implementation.

```golang
// pkg/api/config/v1alpha1/types.go
package v1alpha1

import (
	"time"

	configv1alpha1 "k8s.io/component-base/config/v1alpha1"
)

// ControllerManagerConfiguration defines the embedded RuntimeConfiguration for controller-runtime clients.
type ControllerManagerConfiguration struct {
	Namespace string `json:"namespace,omitempty"`

	SyncPeriod *time.Duration `json:"syncPeriod,omitempty"`

	LeaderElection configv1alpha1.LeaderElectionConfiguration `json:"leaderElection,omitempty"`

	MetricsBindAddress string `json:"metricsBindAddress,omitempty"`

	Health ControllerManagerConfigurationHealth `json:"health,omitempty"`

	Port *int   `json:"port,omitempty"`
	Host string `json:"host,omitempty"`

	CertDir string `json:"certDir,omitempty"`
}

// ControllerManagerConfigurationHealth defines the health configs
type ControllerManagerConfigurationHealth struct {
	HealthProbeBindAddress string `json:"healthProbeBindAddress,omitempty"`

	ReadinessEndpointName string `json:"readinessEndpointName,omitempty"`
	LivenessEndpointName  string `json:"livenessEndpointName,omitempty"`
}
```



#### Default ComponentConfig Type

To enable `controller-runtime` to have a default `ComponentConfig` struct which can be used instead of requiring each controller or extension to build its own `ComponentConfig` type, we can create a `DefaultControllerConfiguration` type which can exist in `pkg/api/config/v1alpha1/types.go`. This will allow the controller authors to use this before needing to implement their own type with additional configs.

```golang
// pkg/api/config/v1alpha1/types.go
package v1alpha1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	configv1alpha1 "sigs.k8s.io/controller-runtime/pkg/apis/config/v1alpha1"
)

// DefaultControllerManagerConfiguration is the Schema for the DefaultControllerManagerConfigurations API
type DefaultControllerManagerConfiguration struct {
	metav1.TypeMeta `json:",inline"`

	Spec   configv1alpha1.ControllerManagerConfiguration   `json:"spec,omitempty"`
}
```

This would allow a controller author to use this struct with any config that supports the json/yaml structure. For example a controller author could define their `Kind` as `FoobarControllerConfiguration` and have it defined as the following.

```yaml
# config.yaml
apiVersion: config.somedomain.io/v1alpha1
kind: FoobarControllerManagerConfiguration
spec:
  port: 9443
  metricsBindAddress: ":8080"
  leaderElection:
    leaderElect: false
```

Given the following config and `DefaultControllerManagerConfiguration` we'd be able to initialize the controller using the following.


```golang
mgr, err := ctrl.NewManagerFromComponentConfig(ctrl.GetConfigOrDie(), scheme, configname, &defaultv1alpha1.DefaultControllerManagerConfiguration{})
if err != nil {
	// ...
}
```

The above example uses `configname` which is the name of the file to load the configuration from and uses `scheme` to get the specific serializer, eg `serializer.NewCodecFactory(scheme)`. This will allow the configuration to be unmarshalled into the `runtime.Object` type and passed into the
`ctrl.NewManagerFromComponentConfig()` as a `ManagerConfiguration` interface.

#### Using Flags w/ ComponentConfig

Since this design still requires setting up the initial `ComponentConfig` type and passing in a pointer to `ctrl.NewFromComponentConfig()` if you want to allow for the use of flags, your controller can use any of the different flagging interfaces. eg [`flag`](https://golang.org/pkg/flag/), [`pflag`](https://pkg.go.dev/github.com/spf13/pflag), [`flagnum`](https://pkg.go.dev/github.com/luci/luci-go/common/flag/flagenum) and set values on the `ComponentConfig` type prior to passing the pointer into the `ctrl.NewFromComponentConfig()`, example below.

```golang
leaderElect := true

config := &defaultv1alpha1.DefaultControllerManagerConfiguration{
	Spec: configv1alpha1.ControllerManagerConfiguration{
		LeaderElection: configv1alpha1.LeaderElectionConfiguration{
			LeaderElect: &leaderElect,
		},
	},
}
mgr, err := ctrl.NewManagerFromComponentConfig(ctrl.GetConfigOrDie(), scheme, configname, config)
if err != nil {
	// ...
}
```

#### Kubebuilder Scaffolding Example

Within expanded in a separate design _(link once created)_ this will allow controller authors to generate a type that implements the `ManagerConfiguration` interface. The following is a sample of what this looks like:

```golang
package config

import (
  "time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	configv1alpha1 "sigs.k8s.io/controller-runtime/pkg/apis/config/v1alpha1"
)

type ControllerNameConfigurationSpec struct {
	configv1alpha1.ControllerManagerConfiguration `json:",inline"`
}

type ControllerNameConfiguration struct {
  metav1.TypeMeta

  Spec ControllerNameConfigurationSpec `json:"spec"`
}
```

Usage of this custom `ComponentConfig` type would require then changing the `ctrl.NewFromComponentConfig()` to use the new struct vs the `DefaultControllerManagerConfiguration`.

## User Stories

### Controller Author with `controller-runtime` and default type

- Mount `ConfigMap`
- Initialize `ctrl.Manager` with `NewFromComponentConfig` with config name and `DefaultControllerManagerConfiguration` type
- Build custom controller as usual

### Controller Author with `controller-runtime` and custom type

- Implement `ComponentConfig` type
- Embed `ControllerManagerConfiguration` type
- Mount `ConfigMap`
- Initialize `ctrl.Manager` with `NewFromComponentConfig` with config name and `ComponentConfig` type
- Build custom controller as usual

### Controller Author with `kubebuilder` (tbd proposal for `kubebuilder`)

- Initialize `kubebuilder` project using `--component-config-name=XYZConfiguration`
- Build custom controller as usual

### Controller User without modifications to config

_Provided that the controller provides manifests_

- Apply the controller to the cluster
- Deploy custom resources

### Controller User with modifications to config

- _Following from previous example without changes_
- Create a new `ConfigMap` for changes
- Modify the `controller-runtime` pod to use the new `ConfigMap`
- Apply the controller to the cluster
- Deploy custom resources


## Risks and Mitigations

- Given that this isn't changing the core Manager initialization for `controller-runtime` it's fairly low risk

## Alternatives

* `NewFromComponentConfig()` could load the object from disk based on the file name and hydrate the `ComponentConfig` type.

## Implementation History

- [x] 02/19/2020: Proposed idea in an issue or [community meeting]
- [x] 02/24/2020: Proposal submitted to `controller-runtime`
- [x] 03/02/2020: Updated with default `DefaultControllerManagerConfiguration`
- [x] 03/04/2020: Updated with embeddable `RuntimeConfig`
- [x] 03/10/2020: Updated embeddable name to `ControllerManagerConfiguration`


<!-- Links -->
[community meeting]: https://docs.google.com/document/d/1Ih-2cgg1bUrLwLVTB9tADlPcVdgnuMNBGbUl4D-0TIk
