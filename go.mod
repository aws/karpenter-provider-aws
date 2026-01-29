module github.com/aws/karpenter-provider-aws

go 1.25.6

// TODO: migrate tablewriter to v1.0.8
// https://github.com/olekukonko/tablewriter/blob/c64d84b3ecc64a18cfc8ba10cdd8c52cc13a7d23/MIGRATION.md?plain=1#L661
replace github.com/olekukonko/tablewriter => github.com/olekukonko/tablewriter v0.0.5

require (
	github.com/Pallinder/go-randomdata v1.2.0
	github.com/PuerkitoBio/goquery v1.11.0
	github.com/avast/retry-go v3.0.0+incompatible
	github.com/aws/amazon-vpc-resource-controller-k8s v1.7.16
	github.com/aws/aws-sdk-go-v2 v1.41.1
	github.com/aws/aws-sdk-go-v2/config v1.32.7
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.17
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.279.2
	github.com/aws/aws-sdk-go-v2/service/eks v1.77.0
	github.com/aws/aws-sdk-go-v2/service/fis v1.37.16
	github.com/aws/aws-sdk-go-v2/service/iam v1.53.2
	github.com/aws/aws-sdk-go-v2/service/pricing v1.40.11
	github.com/aws/aws-sdk-go-v2/service/sqs v1.42.21
	github.com/aws/aws-sdk-go-v2/service/ssm v1.67.8
	github.com/aws/aws-sdk-go-v2/service/sts v1.41.6
	github.com/aws/aws-sdk-go-v2/service/timestreamwrite v1.35.16
	github.com/aws/karpenter-provider-aws/tools/kompat v0.0.0-20240410220356-6b868db24881
	github.com/aws/smithy-go v1.24.0
	github.com/awslabs/amazon-eks-ami/nodeadm v0.0.0-20240229193347-cfab22a10647
	github.com/awslabs/operatorpkg v0.0.0-20251222193911-34e9a1898737
	github.com/awslabs/operatorpkg/aws v0.0.0-20250414225955-b47cd315ffe9
	github.com/docker/docker v28.5.2+incompatible
	github.com/go-logr/zapr v1.3.0
	github.com/google/uuid v1.6.0
	github.com/imdario/mergo v0.3.16
	github.com/jonathan-innis/aws-sdk-go-prometheus v0.1.1
	github.com/mitchellh/hashstructure/v2 v2.0.2
	github.com/onsi/ginkgo/v2 v2.27.5
	github.com/onsi/gomega v1.39.0
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pelletier/go-toml/v2 v2.2.4
	github.com/prometheus/client_golang v1.23.2
	github.com/samber/lo v1.52.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.1
	golang.org/x/sync v0.19.0
	k8s.io/api v0.35.0
	k8s.io/apiextensions-apiserver v0.35.0
	k8s.io/apimachinery v0.35.0
	k8s.io/client-go v0.35.0
	k8s.io/klog/v2 v2.130.1
	k8s.io/utils v0.0.0-20251222233032-718f0e51e6d2
	sigs.k8s.io/controller-runtime v0.22.4
	sigs.k8s.io/karpenter v1.8.1-0.20260106031637-6b1da144076b
	sigs.k8s.io/yaml v1.6.0
)

require (
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.7 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.17 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.17 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.11.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.0.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.13 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.13.0 // indirect
	github.com/evanphx/json-patch v5.9.11+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.9.11 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-openapi/jsonpointer v0.22.4 // indirect
	github.com/go-openapi/jsonreference v0.21.4 // indirect
	github.com/go-openapi/swag v0.25.4 // indirect
	github.com/go-openapi/swag/cmdutils v0.25.4 // indirect
	github.com/go-openapi/swag/conv v0.25.4 // indirect
	github.com/go-openapi/swag/fileutils v0.25.4 // indirect
	github.com/go-openapi/swag/jsonname v0.25.4 // indirect
	github.com/go-openapi/swag/jsonutils v0.25.4 // indirect
	github.com/go-openapi/swag/loading v0.25.4 // indirect
	github.com/go-openapi/swag/mangling v0.25.4 // indirect
	github.com/go-openapi/swag/netutils v0.25.4 // indirect
	github.com/go-openapi/swag/stringutils v0.25.4 // indirect
	github.com/go-openapi/swag/typeutils v0.25.4 // indirect
	github.com/go-openapi/swag/yamlutils v0.25.4 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/gnostic-models v0.7.1 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/pprof v0.0.0-20251213031049-b05bdaca462f // indirect
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.1 // indirect
	github.com/mattn/go-runewidth v0.0.17 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/moby/spdystream v0.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f // indirect
	github.com/olekukonko/tablewriter v1.1.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.67.4 // indirect
	github.com/prometheus/procfs v0.19.2 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/mod v0.31.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/oauth2 v0.34.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/term v0.38.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	golang.org/x/tools v0.40.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.5.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.13.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/cloud-provider v0.35.0 // indirect
	k8s.io/component-base v0.35.0 // indirect
	k8s.io/component-helpers v0.35.0 // indirect
	k8s.io/csi-translation-lib v0.35.0 // indirect
	k8s.io/kube-openapi v0.0.0-20251125145642-4e65d59e963e // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.1 // indirect
)
