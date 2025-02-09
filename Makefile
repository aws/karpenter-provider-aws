CLUSTER_NAME ?= $(shell kubectl config view --minify -o jsonpath='{.clusters[].name}' | rev | cut -d"/" -f1 | rev | cut -d"." -f1)

## Inject the app version into operator.Version
LDFLAGS ?= -ldflags=-X=sigs.k8s.io/karpenter/pkg/operator.Version=$(shell git describe --tags --always | cut -d"v" -f2)

GOFLAGS ?= $(LDFLAGS)
WITH_GOFLAGS = GOFLAGS="$(GOFLAGS)"

## Extra helm options
CLUSTER_ENDPOINT ?= $(shell kubectl config view --minify -o jsonpath='{.clusters[].cluster.server}')
AWS_ACCOUNT_ID ?= $(shell aws sts get-caller-identity --query Account --output text)
KARPENTER_IAM_ROLE_ARN ?= arn:aws:iam::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-karpenter
HELM_OPTS ?= --set serviceAccount.annotations.eks\\.amazonaws\\.com/role-arn=${KARPENTER_IAM_ROLE_ARN} \
      		--set settings.clusterName=${CLUSTER_NAME} \
			--set settings.interruptionQueue=${CLUSTER_NAME} \
			--set settings.avoidEmptySubnets=true \
			--set controller.resources.requests.cpu=1 \
			--set controller.resources.requests.memory=1Gi \
			--set controller.resources.limits.cpu=1 \
			--set controller.resources.limits.memory=1Gi \
			--set settings.featureGates.spotToSpotConsolidation=true \
			--set settings.featureGates.nodeRepair=true \
			--create-namespace

# CR for local builds of Karpenter
KARPENTER_NAMESPACE ?= kube-system
KARPENTER_VERSION ?= $(shell git tag --sort=committerdate | tail -1 | cut -d"v" -f2)
KO_DOCKER_REPO ?= ${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_DEFAULT_REGION}.amazonaws.com/dev
KOCACHE ?= ~/.ko

# Common Directories
MOD_DIRS = $(shell find . -path "./website" -prune -o -name go.mod -type f -print | xargs dirname)
KARPENTER_CORE_DIR = $(shell go list -m -f '{{ .Dir }}' sigs.k8s.io/karpenter)

# TEST_SUITE enables you to select a specific test suite directory to run "make e2etests" against
TEST_SUITE ?= "..."

# Filename when building the binary controller only
GOARCH ?= $(shell go env GOARCH)
BINARY_FILENAME = karpenter-provider-aws-$(GOARCH)

help: ## Display help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

presubmit: verify test ## Run all steps in the developer loop

ci-test: test coverage ## Runs tests and submits coverage

ci-non-test: verify licenses vulncheck ## Runs checks other than tests

run: ## Run Karpenter controller binary against your local cluster
	SYSTEM_NAMESPACE=${KARPENTER_NAMESPACE} \
		KUBERNETES_MIN_VERSION="1.19.0-0" \
		DISABLE_LEADER_ELECTION=true \
		CLUSTER_NAME=${CLUSTER_NAME} \
		INTERRUPTION_QUEUE=${CLUSTER_NAME} \
		FEATURE_GATES="SpotToSpotConsolidation=true" \
		LOG_LEVEL="debug" \
		go run ./cmd/controller/main.go

test: ## Run tests
	go test ./pkg/... \
		-cover -coverprofile=coverage.out -outputdir=. -coverpkg=./... \
		--ginkgo.focus="${FOCUS}" \
		--ginkgo.randomize-all \
		--ginkgo.vv

deflake: ## Run randomized, racing tests until the test fails to catch flakes
	ginkgo \
		--race \
		--focus="${FOCUS}" \
		--randomize-all \
		--until-it-fails \
		-v \
		./pkg/...

e2etests: ## Run the e2e suite against your local cluster
	cd test && CLUSTER_ENDPOINT=${CLUSTER_ENDPOINT} \
		CLUSTER_NAME=${CLUSTER_NAME} \
		INTERRUPTION_QUEUE=${CLUSTER_NAME} \
		go test \
		-p 1 \
		-count 1 \
		-timeout 3.25h \
		-v \
		./suites/$(shell echo $(TEST_SUITE) | tr A-Z a-z)/... \
		--ginkgo.focus="${FOCUS}" \
		--ginkgo.timeout=3h \
		--ginkgo.grace-period=3m \
		--ginkgo.vv

e2etests-deflake: ## Run the e2e suite against your local cluster
	cd test && CLUSTER_NAME=${CLUSTER_NAME} ginkgo \
		--focus="${FOCUS}" \
		--timeout=3h \
		--grace-period=3m \
		--until-it-fails \
		--vv \
		./suites/$(shell echo $(TEST_SUITE) | tr A-Z a-z) \

benchmark:
	go test -tags=test_performance -run=NoTests -bench=. ./...

coverage:
	go tool cover -html coverage.out -o coverage.html

verify: tidy download ## Verify code. Includes dependencies, linting, formatting, etc
	go generate ./...
	hack/boilerplate.sh
	cp  $(KARPENTER_CORE_DIR)/pkg/apis/crds/* pkg/apis/crds
	hack/validation/kubelet.sh
	bash -c 'source ./hack/validation/requirements.sh && injectDomainRequirementRestrictions "karpenter.k8s.aws"'
	bash -c 'source ./hack/validation/labels.sh && injectDomainLabelRestrictions "karpenter.k8s.aws"'
	cp pkg/apis/crds/* charts/karpenter-crd/templates
	hack/mutation/crd_annotations.sh
	hack/github/dependabot.sh
	$(foreach dir,$(MOD_DIRS),cd $(dir) && golangci-lint run $(newline))
	@git diff --quiet ||\
		{ echo "New file modification detected in the Git working tree. Please check in before commit."; git --no-pager diff --name-only | uniq | awk '{print "  - " $$0}'; \
		if [ "${CI}" = true ]; then\
			exit 1;\
		fi;}
	@echo "Validating codegen/docgen build scripts..."
	@find hack/code hack/docs -name "*.go" -type f -print0 | xargs -0 -I {} go build -o /dev/null {}
	actionlint -oneline

vulncheck: ## Verify code vulnerabilities
	@govulncheck ./pkg/...

licenses: download ## Verifies dependency licenses
	# TODO: remove nodeadm check once license is updated
	! go-licenses csv ./... | grep -v -e 'MIT' -e 'Apache-2.0' -e 'BSD-3-Clause' -e 'BSD-2-Clause' -e 'ISC' -e 'MPL-2.0' -e 'github.com/awslabs/amazon-eks-ami/nodeadm'

image: ## Build the Karpenter controller images using ko build
	$(eval CONTROLLER_IMG=$(shell $(WITH_GOFLAGS) KOCACHE=$(KOCACHE) KO_DOCKER_REPO="$(KO_DOCKER_REPO)" ko build --bare github.com/aws/karpenter-provider-aws/cmd/controller))
	$(eval IMG_REPOSITORY=$(shell echo $(CONTROLLER_IMG) | cut -d "@" -f 1 | cut -d ":" -f 1))
	$(eval IMG_TAG=$(shell echo $(CONTROLLER_IMG) | cut -d "@" -f 1 | cut -d ":" -f 2 -s))
	$(eval IMG_DIGEST=$(shell echo $(CONTROLLER_IMG) | cut -d "@" -f 2))

binary: ## Build the Karpenter controller binary using go build
	go build $(GOFLAGS) -o $(BINARY_FILENAME) ./cmd/controller/...

apply: verify image ## Deploy the controller from the current state of your git repository into your ~/.kube/config cluster
	kubectl apply -f ./pkg/apis/crds/
	helm upgrade --install karpenter charts/karpenter --namespace ${KARPENTER_NAMESPACE} \
        $(HELM_OPTS) \
        --set logLevel=debug \
        --set controller.image.repository=$(IMG_REPOSITORY) \
        --set controller.image.tag=$(IMG_TAG) \
        --set controller.image.digest=$(IMG_DIGEST)

install:  ## Deploy the latest released version into your ~/.kube/config cluster
	@echo Upgrading to ${KARPENTER_VERSION}
	helm upgrade --install karpenter oci://public.ecr.aws/karpenter/karpenter --version ${KARPENTER_VERSION} --namespace ${KARPENTER_NAMESPACE} \
		$(HELM_OPTS)

delete: ## Delete the controller from your ~/.kube/config cluster
	helm uninstall karpenter --namespace ${KARPENTER_NAMESPACE}

docgen: ## Generate docs
	KARPENTER_CORE_DIR=$(KARPENTER_CORE_DIR) $(WITH_GOFLAGS) ./hack/docgen.sh

codegen: ## Auto generate files based on AWS APIs response
	$(WITH_GOFLAGS) ./hack/codegen.sh

stable-release-pr: ## Generate PR for stable release
	$(WITH_GOFLAGS) ./hack/release/stable-pr.sh

snapshot: ## Builds and publishes snapshot release
	$(WITH_GOFLAGS) ./hack/release/snapshot.sh

release: ## Builds and publishes stable release
	$(WITH_GOFLAGS) ./hack/release/release.sh

prepare-website: ## prepare the website for release
	./hack/release/prepare-website.sh

toolchain: ## Install developer toolchain
	./hack/toolchain.sh

issues: ## Run GitHub issue analysis scripts
	pip install -r ./hack/github/requirements.txt
	@echo "Set GH_TOKEN env variable to avoid being rate limited by Github"
	./hack/github/feature_request_reactions.py > "karpenter-feature-requests-$(shell date +"%Y-%m-%d").csv"
	./hack/github/label_issue_count.py > "karpenter-labels-$(shell date +"%Y-%m-%d").csv"

website: ## Serve the docs website locally
	cd website && npm install && hugo mod tidy && hugo server

tidy: ## Recursively "go mod tidy" on all directories where go.mod exists
	$(foreach dir,$(MOD_DIRS),cd $(dir) && go mod tidy $(newline))

download: ## Recursively "go mod download" on all directories where go.mod exists
	$(foreach dir,$(MOD_DIRS),cd $(dir) && go mod download $(newline))

update-karpenter: ## Update kubernetes-sigs/karpenter to latest
	go get -u sigs.k8s.io/karpenter@HEAD
	go mod tidy

.PHONY: help presubmit ci-test ci-non-test run test deflake e2etests e2etests-deflake benchmark coverage verify vulncheck licenses image apply install delete docgen codegen stable-release-pr snapshot release prepare-website toolchain issues website tidy download update-karpenter

define newline


endef
