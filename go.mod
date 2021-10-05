module github.com/awslabs/karpenter

go 1.16

require (
	github.com/Pallinder/go-randomdata v1.2.0
	github.com/avast/retry-go v2.7.0+incompatible
	github.com/aws/aws-sdk-go v1.38.69
	github.com/deckarep/golang-set v1.7.1
	github.com/go-logr/zapr v0.4.0
	github.com/imdario/mergo v0.3.12
	github.com/mitchellh/hashstructure/v2 v2.0.2
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.15.0
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/prometheus/client_golang v1.11.0
	go.uber.org/multierr v1.7.0
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac
	k8s.io/api v0.21.4
	k8s.io/apimachinery v0.21.4
	k8s.io/client-go v0.21.4
	knative.dev/pkg v0.0.0-20211005203644-a00ba487121e
	sigs.k8s.io/controller-runtime v0.9.7
)
