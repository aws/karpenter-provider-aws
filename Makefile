export K8S_VERSION ?= 1.23.x
export KUBEBUILDER_ASSETS ?= ${HOME}/.kubebuilder/bin

## Inject the app version into project.Version
LDFLAGS ?= -ldflags=-X=github.com/aws/karpenter/pkg/utils/project.Version=$(shell git describe --tags --always)
GOFLAGS ?= $(LDFLAGS)
WITH_GOFLAGS = GOFLAGS="$(GOFLAGS)"

## Extra helm options
CLUSTER_NAME ?= $(shell kubectl config view --minify -o jsonpath='{.clusters[].name}' | rev | cut -d"/" -f1 | rev | cut -d"." -f1)
CLUSTER_ENDPOINT ?= $(shell kubectl config view --minify -o jsonpath='{.clusters[].cluster.server}')
AWS_ACCOUNT_ID ?= $(shell aws sts get-caller-identity --query Account --output text)
KARPENTER_IAM_ROLE_ARN ?= arn:aws:iam::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-karpenter
HELM_OPTS ?= --set serviceAccount.annotations.eks\\.amazonaws\\.com/role-arn=${KARPENTER_IAM_ROLE_ARN} \
      		--set clusterName=${CLUSTER_NAME} \
			--set clusterEndpoint=${CLUSTER_ENDPOINT} \
			--set aws.defaultInstanceProfile=KarpenterNodeInstanceProfile-${CLUSTER_NAME} \
			--create-namespace
TEST_FILTER ?= .*

# CR for local builds of Karpenter
SYSTEM_NAMESPACE ?= karpenter
KO_DOCKER_REPO ?= ${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_DEFAULT_REGION}.amazonaws.com/karpenter
GETTING_STARTED_SCRIPT_DIR = website/content/en/preview/getting-started/getting-started-with-eksctl/scripts

help: ## Display help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

dev: verify test ## Run all steps in the developer loop

ci: toolchain verify licenses battletest coverage ## Run all steps used by continuous integration

run: ## Run Karpenter controller binary against your local cluster
	SYSTEM_NAMESPACE=${SYSTEM_NAMESPACE} go run ./cmd/controller/main.go \
		--cluster-name=${CLUSTER_NAME} \
		--cluster-endpoint=${CLUSTER_ENDPOINT} \
		--aws-default-instance-profile=KarpenterNodeInstanceProfile-${CLUSTER_NAME} \
		--leader-elect=false

test: ## Run tests
	go test -run=${TEST_FILTER} ./pkg/...

battletest: ## Run randomized, racing, code coveraged, tests
	go test -run=${TEST_FILTER} ./pkg/... \
		-race \
		-cover -coverprofile=coverage.out -outputdir=. -coverpkg=./pkg/... \
		-ginkgo.randomizeAllSpecs \
		-tags random_test_delay

e2etests: ## Run the e2e suite against your local cluster
	go clean -testcache
	go test -p 1 -timeout 180m -v ./test/suites/... -run=${TEST_FILTER}

benchmark:
	go test -tags=test_performance -run=NoTests -bench=. ./...

deflake:
	for i in $(shell seq 1 5); do make battletest || exit 1; done

coverage:
	go tool cover -html coverage.out -o coverage.html

verify: codegen ## Verify code. Includes dependencies, linting, formatting, etc
	go mod tidy
	go mod download
	golangci-lint run
	@git diff --quiet ||\
		{ echo "New file modification detected in the Git working tree. Please check in before commit.";\
		if [ $(MAKECMDGOALS) = 'ci' ]; then\
			exit 1;\
		fi;}
	@echo "Validating codegen/docgen build scripts..."
	@find hack/code hack/docs -name "*.go" -type f -exec go build -o /dev/null {} \;
	@govulncheck ./pkg/...

licenses: ## Verifies dependency licenses
	go mod download
	! go-licenses csv ./... | grep -v -e 'MIT' -e 'Apache-2.0' -e 'BSD-3-Clause' -e 'BSD-2-Clause' -e 'ISC' -e 'MPL-2.0'

setup: ## Sets up the IAM roles needed prior to deploying the karpenter-controller. This command only needs to be run once
	./$(GETTING_STARTED_SCRIPT_DIR)/add-roles.sh $(KARPENTER_VERSION)

build: ## Build the Karpenter controller and webhook images using ko build
	$(eval CONTROLLER_IMG=$(shell $(WITH_GOFLAGS) ko build -B github.com/aws/karpenter/cmd/controller))
	$(eval WEBHOOK_IMG=$(shell $(WITH_GOFLAGS) ko build -B github.com/aws/karpenter/cmd/webhook))

apply: build ## Deploy the controller from the current state of your git repository into your ~/.kube/config cluster
	helm upgrade --install karpenter charts/karpenter --namespace karpenter \
		$(HELM_OPTS) \
		--set controller.image=$(CONTROLLER_IMG) \
		--set webhook.image=$(WEBHOOK_IMG)

install:  ## Deploy the latest released version into your ~/.kube/config cluster
	@echo Upgrading to $(shell grep version charts/karpenter/Chart.yaml)
	helm upgrade --install karpenter charts/karpenter --namespace karpenter \
		$(HELM_OPTS)

delete: ## Delete the controller from your ~/.kube/config cluster
	helm uninstall karpenter --namespace karpenter

codegen: ## Generate code. Must be run if changes are made to ./pkg/apis/...
	controller-gen \
		object:headerFile="hack/boilerplate.go.txt" \
		crd \
		paths="./pkg/..." \
		output:crd:artifacts:config=charts/karpenter/crds
	hack/boilerplate.sh

docgen: ## Generate docs
	go run hack/docs/metrics_gen_docs.go pkg/ website/content/en/preview/tasks/metrics.md
	go run hack/docs/instancetypes_gen_docs.go website/content/en/preview/AWS/instance-types.md
	go run hack/docs/configuration_gen_docs.go website/content/en/preview/tasks/configuration.md

release-gen: docgen ## Generate any materials which should be updated prior to release
	go run hack/code/prices_gen.go -- pkg/cloudprovider/aws/zz_generated.pricing.go
	go run hack/code/vpc_limits_gen.go -- \
		--url=https://raw.githubusercontent.com/aws/amazon-vpc-resource-controller-k8s/master/pkg/aws/vpc/limits.go \
		--output=pkg/cloudprovider/aws/zz_generated.vpclimits.go
	hack/boilerplate.sh
	go mod tidy
	go mod download
	golangci-lint run

release: release-gen ## Generate release manifests and publish a versioned container image.
	$(WITH_GOFLAGS) ./hack/release.sh

nightly: ## Tag the latest snapshot release with timestamp
	./hack/add-snapshot-tag.sh $(shell git rev-parse HEAD) $(shell date +"%Y%m%d") "nightly"

snapshot: ## Generate a snapshot release out of the current commit
	$(WITH_GOFLAGS) ./hack/snapshot.sh

stablerelease: ## Tags the snapshot release of the current commit with the latest tag available, for prod launch
	./hack/add-snapshot-tag.sh $(shell git rev-parse HEAD) $(shell git describe --tags --exact-match || echo "Current commit is not tagged") "stable"

toolchain: ## Install developer toolchain
	./hack/toolchain.sh

issues: ## Run GitHub issue analysis scripts
	pip install -r ./hack/github/requirements.txt
	@echo "Set GH_TOKEN env variable to avoid being rate limited by Github"
	./hack/github/feature_request_reactions.py > "karpenter-feature-requests-$(shell date +"%Y-%m-%d").csv"
	./hack/github/label_issue_count.py > "karpenter-labels-$(shell date +"%Y-%m-%d").csv"

website: ## Serve the docs website locally
	cd website && npm install && git submodule update --init --recursive && hugo server

.PHONY: help dev ci release test battletest verify codegen docgen apply delete toolchain release release-gen licenses issues website nightly snapshot e2etests
