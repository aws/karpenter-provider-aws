# Image URL to use all building/pushing image targets
IMG ?= ${KO_DOCKER_REPO}/karpenter:latest

help: ## Display help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

ci: generate verify battletest ## Run all steps used by continuous integration

test: ## Run tests
	ginkgo -r

battletest: ## Run stronger tests
	# Ensure all files have cyclo-complexity =< 10
	gocyclo -over 10 ./pkg
	# Run randomized, parallelized, racing, code coveraged, tests
	ginkgo -r \
		-cover -coverprofile=coverage.out -outputdir=. -coverpkg=./pkg/... \
		--randomizeAllSpecs --randomizeSuites -race
	go tool cover -func coverage.out

verify: ## Verify code. Includes dependencies, linting, formatting, etc
	go mod tidy
	go mod download
	go vet ./...
	go fmt ./...
	golangci-lint run

generate: ## Generate code. Must be run if changes are made to ./pkg/apis/...
	controller-gen \
		object:headerFile="hack/boilerplate.go.txt" \
		webhook \
		crd:trivialVersions=false \
		rbac:roleName=karpenter \
		paths="./pkg/..." \
		output:crd:artifacts:config=config/crd/bases \
		output:webhook:artifacts:config=config/webhook

	./hack/boilerplate.sh

	# Hack to remove v1.AdmissionReview until https://github.com/kubernetes-sigs/controller-runtime/issues/1161 is fixed
	perl -pi -e 's/^  - v1$$//g' config/webhook/manifests.yaml
	# CRDs don't currently jive with volatile time.
	# `properties[lastTransitionTime].type: Unsupported value: "Any": supported
	# values: "array", "boolean", "integer", "number", "object", "string"`
	perl -pi -e 's/Any/string/g' config/crd/bases/autoscaling.karpenter.sh_horizontalautoscalers.yaml
	perl -pi -e 's/Any/string/g' config/crd/bases/autoscaling.karpenter.sh_scalablenodegroups.yaml
	perl -pi -e 's/Any/string/g' config/crd/bases/autoscaling.karpenter.sh_metricsproducers.yaml

apply: ## Deploy the controller from your ~/.kube/config cluster
	kubectl kustomize config/dev | ko apply -B -f -

delete: ## Delete the controller from your ~/.kube/config cluster
	kubectl kustomize config/dev | ko delete -f -

release: ## Build and release a container image to $KO_DOCKER_REPO/karpenter
	docker build . -t ${IMG}
	docker push ${IMG}

toolchain: ## Install developer toolchain
	./hack/toolchain.sh

.PHONY: help all test release run apply delete verify generate toolchain
