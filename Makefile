RELEASE_REPO ?= public.ecr.aws/karpenter
RELEASE_VERSION ?= $(shell git describe --tags --always)
RELEASE_PLATFORM ?= --platform=linux/amd64,linux/arm64

## Inject these annotations to cosign signing
DATE_FMT = +'%Y-%m-%dT%H:%M:%SZ'
SOURCE_DATE_EPOCH ?= $(shell git log -1 --pretty=%ct)
ifdef SOURCE_DATE_EPOCH
    BUILD_DATE ?= $(shell date -u -d "@$(SOURCE_DATE_EPOCH)" "$(DATE_FMT)" 2>/dev/null || date -u -r "$(SOURCE_DATE_EPOCH)" "$(DATE_FMT)" 2>/dev/null || date -u "$(DATE_FMT)")
else
    BUILD_DATE ?= $(shell date "$(DATE_FMT)")
endif
COSIGN_FLAGS ?= -a GIT_HASH=$(shell git rev-parse HEAD) -a GIT_VERSION=${RELEASE_VERSION} -a BUILD_DATE=${BUILD_DATE}

## Inject the app version into project.Version
LDFLAGS ?= "-ldflags=-X=github.com/aws/karpenter/pkg/utils/project.Version=$(RELEASE_VERSION)"
GOFLAGS ?= "-tags=$(CLOUD_PROVIDER) $(LDFLAGS)"
WITH_GOFLAGS = GOFLAGS=$(GOFLAGS)
WITH_RELEASE_REPO = KO_DOCKER_REPO=$(RELEASE_REPO)

## Extra helm options
CLUSTER_NAME ?= $(shell kubectl config view --minify -o jsonpath='{.clusters[].name}' | rev | cut -d"/" -f1 | rev)
CLUSTER_ENDPOINT ?= $(shell kubectl config view --minify -o jsonpath='{.clusters[].cluster.server}')
HELM_OPTS ?= --set controller.clusterName=${CLUSTER_NAME} --set controller.clusterEndpoint=${CLUSTER_ENDPOINT}

help: ## Display help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

dev: verify test ## Run all steps in the developer loop

ci: verify licenses battletest ## Run all steps used by continuous integration

release: verify publish helm website ## Run all steps in release workflow

test: ## Run tests
	ginkgo -r

battletest: ## Run stronger tests
	# Ensure all files have cyclo-complexity =< 10
	gocyclo -over 11 ./pkg
	# Run randomized, parallelized, racing, code coveraged, tests
	ginkgo -r \
		-cover -coverprofile=coverage.out -outputdir=. -coverpkg=./pkg/... \
		--randomizeAllSpecs --randomizeSuites -race
	go tool cover -html coverage.out -o coverage.html

verify: codegen ## Verify code. Includes dependencies, linting, formatting, etc
	go mod tidy
	go mod download
	go vet ./...
	go fmt ./...
	golangci-lint run
	@git diff --quiet ||\
		{ echo "New file modification detected in the Git working tree. Please check in before commit.";\
		if [ $(MAKECMDGOALS) = 'ci' ]; then\
			exit 1;\
		fi;}

licenses: ## Verifies dependency licenses and requires GITHUB_TOKEN to be set
	go build $(GOFLAGS) -o karpenter cmd/controller/main.go
	golicense hack/license-config.hcl karpenter

apply: ## Deploy the controller into your ~/.kube/config cluster
	helm template --include-crds  karpenter charts/karpenter --namespace karpenter \
		$(HELM_OPTS) \
		--set controller.image=ko://github.com/aws/karpenter/cmd/controller \
		--set webhook.image=ko://github.com/aws/karpenter/cmd/webhook \
		| $(WITH_GOFLAGS) ko apply -B -f -

delete: ## Delete the controller from your ~/.kube/config cluster
	helm template karpenter charts/karpenter --namespace karpenter \
		$(HELM_OPTS) \
		--set serviceAccount.create=false \
		| kubectl delete -f -

codegen: ## Generate code. Must be run if changes are made to ./pkg/apis/...
	controller-gen \
		object:headerFile="hack/boilerplate.go.txt" \
		crd \
		paths="./pkg/..." \
		output:crd:artifacts:config=charts/karpenter/crds
	hack/boilerplate.sh

publish: ## Generate release manifests and publish a versioned container image.
	@aws ecr-public get-login-password --region us-east-1 | docker login --username AWS --password-stdin $(RELEASE_REPO)
	yq e -i ".controller.image = \"$$($(WITH_RELEASE_REPO) $(WITH_GOFLAGS) ko publish -B -t $(RELEASE_VERSION) $(RELEASE_PLATFORM) ./cmd/controller)\"" charts/karpenter/values.yaml
	yq e -i ".webhook.image = \"$$($(WITH_RELEASE_REPO) $(WITH_GOFLAGS) ko publish -B -t $(RELEASE_VERSION) $(RELEASE_PLATFORM) ./cmd/webhook)\"" charts/karpenter/values.yaml
	yq e -i '.version = "$(subst v,,${RELEASE_VERSION})"' charts/karpenter/Chart.yaml
	COSIGN_EXPERIMENTAL=1 cosign sign ${COSIGN_FLAGS} ${RELEASE_REPO}/controller:${RELEASE_VERSION}
	COSIGN_EXPERIMENTAL=1 cosign sign ${COSIGN_FLAGS} ${RELEASE_REPO}/webhook:${RELEASE_VERSION}

helm: ## Generate Helm Chart
	cd charts;helm lint karpenter;helm package karpenter;helm repo index .
	helm-docs

website: ## Generate Docs Website
	rsync -a website/content/en/docs/ website/content/en/${RELEASE_VERSION}-docs

toolchain: ## Install developer toolchain
	./hack/toolchain.sh

.PHONY: help dev ci release test battletest verify codegen apply delete publish helm website toolchain licenses
