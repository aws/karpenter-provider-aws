module github.com/ellistarn/karpenter

go 1.14

require (
	github.com/aws/aws-sdk-go v1.34.10
	github.com/go-logr/zapr v0.1.1
	github.com/golangci/golangci-lint v1.31.0
	github.com/google/ko v0.5.2
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.1
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.0.0
	go.uber.org/zap v1.11.0
	k8s.io/api v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v0.18.6
	k8s.io/code-generator v0.18.6
	sigs.k8s.io/controller-runtime v0.6.2
	sigs.k8s.io/controller-tools v0.3.1-0.20200528125929-5c0c6ae3b64b
	sigs.k8s.io/kubebuilder v1.0.9-0.20200321200244-8b53abeb4280
)
