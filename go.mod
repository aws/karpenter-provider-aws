module github.com/awslabs/karpenter

go 1.15

require (
	github.com/Pallinder/go-randomdata v1.2.0
	github.com/aws/aws-sdk-go v1.35.12
	github.com/go-logr/zapr v0.2.0
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.3
	github.com/prometheus/client_golang v1.8.0
	github.com/prometheus/common v0.14.0
	github.com/robfig/cron/v3 v3.0.0
	go.uber.org/multierr v1.6.0
	go.uber.org/zap v1.16.0
	k8s.io/api v0.19.3
	k8s.io/apimachinery v0.19.3
	k8s.io/client-go v0.19.3
	knative.dev/pkg v0.0.0-20191217184203-cf220a867b3d
	sigs.k8s.io/controller-runtime v0.7.0-alpha.3
	sigs.k8s.io/yaml v1.2.0
)
