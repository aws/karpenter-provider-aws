module github.com/ellistarn/karpenter

go 1.14

require (
	github.com/aws/aws-sdk-go v1.34.10
	github.com/go-logr/zapr v0.1.0
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.1
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.0.0
	go.uber.org/zap v1.10.0
	k8s.io/api v0.18.4
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v0.18.4
	sigs.k8s.io/controller-runtime v0.6.1
)
