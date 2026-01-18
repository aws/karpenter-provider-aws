module github.com/awslabs/amazon-eks-ami/nodeadm

go 1.21

require (
	github.com/aws/aws-sdk-go v1.50.6
	github.com/aws/aws-sdk-go-v2/config v1.26.6
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.14.11
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.146.0
	github.com/aws/aws-sdk-go-v2/service/ecr v1.24.7
	github.com/aws/smithy-go v1.19.0
	github.com/containerd/containerd v1.7.13
	github.com/coreos/go-systemd/v22 v22.5.0
	github.com/integrii/flaggy v1.5.2
	github.com/stretchr/testify v1.8.4
	go.uber.org/zap v1.26.0
	golang.org/x/mod v0.14.0
	k8s.io/apimachinery v0.29.1
	k8s.io/cri-api v0.29.1
	k8s.io/kubelet v0.29.1
	sigs.k8s.io/controller-runtime v0.17.0
)

require (
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	golang.org/x/tools v0.16.1 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230822172742-b8732ec3820d // indirect
	google.golang.org/grpc v1.58.3 // indirect
	k8s.io/component-base v0.29.1 // indirect
)

require dario.cat/mergo v1.0.0 // direct

require (
	github.com/aws/aws-sdk-go-v2 v1.24.1 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.16.16 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.2.10 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.5.10 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.7.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.10.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.10.10 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.18.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.21.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.26.7 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/matttproud/golang_protobuf_extensions/v2 v2.0.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_golang v1.18.0 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.45.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/spf13/cobra v1.7.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.19.0 // indirect
	golang.org/x/sys v0.16.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/api v0.29.1
	k8s.io/klog/v2 v2.110.1 // indirect
	k8s.io/utils v0.0.0-20240102154912-e7106e64919e // direct
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)
