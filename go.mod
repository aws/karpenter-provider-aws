module github.com/ellistarn/karpenter

go 1.14

require (
	github.com/Pallinder/go-randomdata v1.2.0
	github.com/aws/aws-sdk-go v1.34.10
	github.com/cloudevents/sdk-go v1.2.0
	github.com/fzipp/gocyclo v0.0.0-20150627053110-6acd4345c835
	github.com/go-logr/zapr v0.1.1
	github.com/golangci/golangci-lint v1.31.0
	github.com/google/ko v0.6.0
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/prometheus/client_golang v1.7.1
	github.com/prometheus/common v0.10.0
	go.uber.org/multierr v1.6.0
	go.uber.org/zap v1.11.0
	k8s.io/api v0.18.8
	k8s.io/apimachinery v0.18.8
	k8s.io/client-go v0.18.8
	knative.dev/pkg v0.0.0-20191217184203-cf220a867b3d
	sigs.k8s.io/controller-runtime v0.6.2
	sigs.k8s.io/controller-tools v0.4.0
	sigs.k8s.io/kubebuilder v1.0.9-0.20200321200244-8b53abeb4280
	sigs.k8s.io/yaml v1.2.0
)
