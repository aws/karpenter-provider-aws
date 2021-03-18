GOFLAGS ?= "-tags=${CLOUD_PROVIDER}"

RELEASE_REPO ?= public.ecr.aws/b6u6q9h4
RELEASE_VERSION ?= v0.1.3
RELEASE_MANIFEST = releases/${CLOUD_PROVIDER}/manifest.yaml

WITH_GOFLAGS = GOFLAGS=${GOFLAGS}
WITH_RELEASE_REPO = KO_DOCKER_REPO=${RELEASE_REPO}

help: ## Display help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

dev: verify test ## Run all steps in the developer loop

ci: verify battletest ## Run all steps used by continuous integration

release: publish helm docs ## Run all steps in release workflow

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
	golangci-lint run

apply: ## Deploy the controller into your ~/.kube/config cluster
	$(WITH_GOFLAGS) ko apply -B -f config

delete: ## Delete the controller from your ~/.kube/config cluster
	ko delete -f config

codegen: ## Generate code. Must be run if changes are made to ./pkg/apis/...
	./hack/codegen.sh

publish: ## Generate release manifests and publish a versioned container image.
	$(WITH_RELEASE_REPO) $(WITH_GOFLAGS) ko resolve -B -t $(RELEASE_VERSION) -f config > $(RELEASE_MANIFEST)

helm: ## Generate Helm Chart
	cp $(RELEASE_MANIFEST) charts/karpenter/templates
	yq e -i '.version = "$(RELEASE_VERSION)"' ./charts/karpenter/Chart.yaml
	cd charts; helm package karpenter; helm repo index .

docs: ## Generate Docs
	gen-crd-api-reference-docs \
		-api-dir ./pkg/apis/provisioning/v1alpha1 \
		-config $(shell go env GOMODCACHE)/github.com/ahmetb/gen-crd-api-reference-docs@v0.2.0/example-config.json \
		-out-file docs/README.md \
		-template-dir $(shell go env GOMODCACHE)/github.com/ahmetb/gen-crd-api-reference-docs@v0.2.0/template

toolchain: ## Install developer toolchain
	./hack/toolchain.sh

.PHONY: help dev ci release test battletest verify codegen apply delete publish helm docs toolchain
