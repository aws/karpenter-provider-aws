module github.com/ellistarn/karpenter

go 1.14

require (
	github.com/aws/aws-sdk-go v1.34.10
	github.com/go-logr/zapr v0.1.0
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/onsi/ginkgo v1.14.0
	github.com/onsi/gomega v1.10.1
	go.uber.org/zap v1.10.0
	k8s.io/api v0.18.4
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v0.18.4
	k8s.io/component-base v0.18.4
	sigs.k8s.io/controller-runtime v0.6.1
)
