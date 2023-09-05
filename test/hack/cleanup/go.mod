module github.com/aws/karpenter/test/hack/cleanup

go 1.21

require (
	github.com/aws/aws-sdk-go v1.44.309
	github.com/aws/aws-sdk-go-v2/config v1.18.27
	github.com/aws/aws-sdk-go-v2/service/cloudformation v1.30.0
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.102.0
	github.com/aws/aws-sdk-go-v2/service/iam v1.21.0
	github.com/aws/aws-sdk-go-v2/service/timestreamwrite v1.18.2
	github.com/samber/lo v1.38.1
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.24.0
	k8s.io/client-go v0.27.4
)

require (
	github.com/aws/aws-sdk-go-v2 v1.20.1 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.13.26 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.13.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.38 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.4.32 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.35 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.7.32 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.9.28 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.12.12 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.14.12 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.19.2 // indirect
	github.com/aws/smithy-go v1.14.1 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	golang.org/x/exp v0.0.0-20220303212507-bbda1eaf7a17 // indirect
	golang.org/x/time v0.0.0-20220210224613-90d013bbcef8 // indirect
	k8s.io/apimachinery v0.27.4 // indirect
	k8s.io/klog/v2 v2.90.1 // indirect
	k8s.io/utils v0.0.0-20230209194617-a36077c30491 // indirect
)
