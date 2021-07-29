module github.com/awslabs/karpenter

go 1.16

require (
	bou.ke/monkey v1.0.2
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
	go.uber.org/multierr v1.7.0
	go.uber.org/zap v1.18.1 // indirect
	golang.org/x/time v0.0.0-20210611083556-38a9dc6acbc6
	k8s.io/api v0.20.7
	k8s.io/apimachinery v0.20.7
	k8s.io/client-go v0.20.7
	knative.dev/pkg v0.0.0-20210628225612-51cfaabbcdf6
	sigs.k8s.io/controller-runtime v0.8.3
)
