package registry

import (
	"context"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/utils/log"
	"knative.dev/pkg/apis"
)

func New(options cloudprovider.Options) cloudprovider.Factory {
	cloudProvider := NewCloudProvider(options)
	RegisterOrDie(cloudProvider)
	return cloudProvider
}

// RegisterOrDie populates supported instance types, zones, operating systems,
// architectures, and validation logic. This operation should only be called
// once at startup time. Typically, this call is made by registry.New(), but
// must be called if the cloud provider is constructed directly (e.g. tests).
func RegisterOrDie(factory cloudprovider.Factory) {
	zones := map[string]bool{}
	architectures := map[string]bool{}
	operatingSystems := map[string]bool{}

	// TODO remove the factory pattern and avoid passing an empty provisioner
	instanceTypes, err := factory.CapacityFor(&v1alpha1.Provisioner{}).GetInstanceTypes(context.Background())
	log.PanicIfError(err, "Failed to retrieve instance types")
	for _, instanceType := range instanceTypes {
		v1alpha1.SupportedInstanceTypes = append(v1alpha1.SupportedInstanceTypes, instanceType.Name())
		for _, zone := range instanceType.Zones() {
			zones[zone] = true
		}
		for _, architecture := range instanceType.Architectures() {
			architectures[architecture] = true
		}
		for _, operatingSystem := range instanceType.OperatingSystems() {
			operatingSystems[operatingSystem] = true
		}
	}
	for zone := range zones {
		v1alpha1.SupportedZones = append(v1alpha1.SupportedZones, zone)
	}
	for architecture := range architectures {
		v1alpha1.SupportedArchitectures = append(v1alpha1.SupportedArchitectures, architecture)
	}
	for operatingSystem := range operatingSystems {
		v1alpha1.SupportedOperatingSystems = append(v1alpha1.SupportedOperatingSystems, operatingSystem)
	}
	v1alpha1.ValidationHook = func(ctx context.Context, spec *v1alpha1.ProvisionerSpec) *apis.FieldError {
		return factory.CapacityFor(&v1alpha1.Provisioner{Spec: *spec}).Validate(ctx)
	}
}
