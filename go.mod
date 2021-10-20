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
	github.com/onsi/gomega v1.13.0
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/prometheus/client_golang v1.11.0
	go.uber.org/multierr v1.7.0
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/time v0.0.0-20210611083556-38a9dc6acbc6
	golang.org/x/tools v0.1.6-0.20210908190839-cf92b39a962c // indirect
	k8s.io/api v0.20.7
	k8s.io/apimachinery v0.20.7
	k8s.io/client-go v0.20.7
	knative.dev/pkg v0.0.0-20210628225612-51cfaabbcdf6
	sigs.k8s.io/controller-runtime v0.8.3
)
