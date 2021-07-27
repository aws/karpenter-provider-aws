RELEASE_REPO ?= public.ecr.aws/karpenter
RELEASE_VERSION ?= $(shell git describe --tags --always)

## Inject the app version into project.Version
LDFLAGS ?= "-ldflags=-X=github.com/awslabs/karpenter/pkg/utils/project.Version=$(RELEASE_VERSION)"
GOFLAGS ?= "-tags=$(CLOUD_PROVIDER) $(LDFLAGS)"
WITH_GOFLAGS = GOFLAGS=$(GOFLAGS)
WITH_RELEASE_REPO = KO_DOCKER_REPO=$(RELEASE_REPO)

## Extra helm options
HELM_OPTS ?=

help: ## Display help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

dev: verify test ## Run all steps in the developer loop

ci: verify licenses battletest ## Run all steps used by continuous integration

release: codegen publish helm docs ## Run all steps in release workflow

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

verify: ## Verify code. Includes dependencies, linting, formatting, etc
	go mod tidy
	go mod download
	go vet ./...
	go fmt ./...
	golangci-lint run --timeout 5m

licenses: ## Verifies dependency licenses and requires GITHUB_TOKEN to be set
	go build $(GOFLAGS) -o karpenter cmd/controller/main.go
	golicense hack/license-config.hcl karpenter

apply: ## Deploy the controller into your ~/.kube/config cluster
	helm template karpenter charts/karpenter --namespace karpenter \
		$(HELM_OPTS) \
		--set controller.image=ko://github.com/awslabs/karpenter/cmd/controller \
		--set webhook.image=ko://github.com/awslabs/karpenter/cmd/webhook \
		| $(WITH_GOFLAGS) ko apply -B -f -

delete: ## Delete the controller from your ~/.kube/config cluster
	helm template karpenter charts/karpenter --namespace karpenter | kubectl delete -f -

codegen: ## Generate code. Must be run if changes are made to ./pkg/apis/...
	controller-gen \
		object:headerFile="hack/boilerplate.go.txt" \
		crd:trivialVersions=false \
		paths="./pkg/..." \
		output:crd:artifacts:config=charts/karpenter/templates
	# CRDs don't currently jive with VolatileTime, which has an Any type.
	perl -pi -e 's/Any/string/g' charts/karpenter/templates/karpenter.sh_provisioners.yaml
	hack/boilerplate.sh
	gen-crd-api-reference-docs \
		-api-dir ./pkg/apis/provisioning/v1alpha3 \
		-config $(shell go env GOMODCACHE)/github.com/ahmetb/gen-crd-api-reference-docs@v0.3.0/example-config.json \
		-out-file API.md \
		-template-dir $(shell go env GOMODCACHE)/github.com/ahmetb/gen-crd-api-reference-docs@v0.3.0/template

publish: ## Generate release manifests and publish a versioned container image.
	@aws ecr-public get-login-password --region us-east-1 | docker login --username AWS --password-stdin $(RELEASE_REPO)
	yq e -i ".controller.image = \"$$($(WITH_RELEASE_REPO) $(WITH_GOFLAGS) ko publish -B -t $(RELEASE_VERSION) --platform all ./cmd/controller)\"" charts/karpenter/values.yaml
	yq e -i ".webhook.image = \"$$($(WITH_RELEASE_REPO) $(WITH_GOFLAGS) ko publish -B -t $(RELEASE_VERSION) --platform all ./cmd/webhook)\"" charts/karpenter/values.yaml
	yq e -i '.version = "$(RELEASE_VERSION)"' charts/karpenter/Chart.yaml

helm: ## Generate Helm Chart
	cd charts;helm lint karpenter;helm package karpenter;helm repo index .

website: ## Generate Docs Website
	cd website; npm install; git submodule update --init --recursive; hugo

toolchain: ## Install developer toolchain
	./hack/toolchain.sh

.PHONY: help dev ci release test battletest verify codegen apply delete publish helm website toolchain licenses
