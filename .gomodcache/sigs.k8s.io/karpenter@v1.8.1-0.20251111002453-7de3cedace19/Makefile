# This is the format of an AWS ECR Public Repo as an example.
export KWOK_REPO ?= ${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_DEFAULT_REGION}.amazonaws.com
export KARPENTER_NAMESPACE=kube-system

HELM_OPTS ?= --set logLevel=debug \
			--set controller.resources.requests.cpu=1 \
			--set controller.resources.requests.memory=1Gi \
			--set controller.resources.limits.cpu=1 \
			--set controller.resources.limits.memory=1Gi \
			--set settings.featureGates.nodeRepair=true \
			--set settings.featureGates.staticCapacity=true

help: ## Display help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

presubmit: verify test licenses vulncheck ## Run all steps required for code to be checked in

install-kwok: ## Install kwok provider
	./hack/install-kwok.sh

uninstall-kwok: ## Uninstall kwok provider
	UNINSTALL=true ./hack/install-kwok.sh

build-with-kind: # build with kind assumes the image will be uploaded directly onto the kind control plane, without an image repository
	$(eval CONTROLLER_IMG=$(shell $(WITH_GOFLAGS) KO_DOCKER_REPO="$(KWOK_REPO)" ko build sigs.k8s.io/karpenter/kwok))
	$(eval IMG_REPOSITORY=$(shell echo $(CONTROLLER_IMG) | cut -d ":" -f 1))
	$(eval IMG_TAG=latest)

build: ## Build the Karpenter KWOK controller images using ko build
	$(eval CONTROLLER_IMG=$(shell $(WITH_GOFLAGS) KO_DOCKER_REPO="$(KWOK_REPO)" ko build -B sigs.k8s.io/karpenter/kwok))
	$(eval IMG_REPOSITORY=$(shell echo $(CONTROLLER_IMG) | cut -d "@" -f 1 | cut -d ":" -f 1))
	$(eval IMG_TAG=$(shell echo $(CONTROLLER_IMG) | cut -d "@" -f 1 | cut -d ":" -f 2 -s))
	$(eval IMG_DIGEST=$(shell echo $(CONTROLLER_IMG) | cut -d "@" -f 2))

apply-with-kind: verify build-with-kind ## Deploy the kwok controller from the current state of your git repository into your ~/.kube/config cluster
	kubectl apply -f kwok/charts/crds
	helm upgrade --install karpenter kwok/charts --namespace $(KARPENTER_NAMESPACE) --skip-crds \
		$(HELM_OPTS) \
		--set controller.image.repository=$(IMG_REPOSITORY) \
		--set controller.image.tag=$(IMG_TAG) \
		--set serviceMonitor.enabled=true \
		--set-string controller.env[0].name=ENABLE_PROFILING \
		--set-string controller.env[0].value=true

JUNIT_REPORT := $(if $(ARTIFACT_DIR), --ginkgo.junit-report="$(ARTIFACT_DIR)/junit_report.xml")
e2etests: ## Run the e2e suite against your local cluster
	cd test && go test \
		-count 1 \
		-timeout 2h \
		-v \
		./suites/$(shell echo $(TEST_SUITE) | tr A-Z a-z)/... \
		$(JUNIT_REPORT) \
		--ginkgo.focus="${FOCUS}" \
		--ginkgo.skip="${SKIP}" \
		--ginkgo.timeout=2h \
		--ginkgo.grace-period=5m \
		--ginkgo.vv

# Run make install-kwok to install the kwok controller in your cluster first
# Webhooks are currently not supported in the kwok provider.
apply: verify build ## Deploy the kwok controller from the current state of your git repository into your ~/.kube/config cluster
	kubectl apply -f kwok/charts/crds
	helm upgrade --install karpenter kwok/charts --namespace $(KARPENTER_NAMESPACE) --skip-crds \
		$(HELM_OPTS) \
		--set controller.image.repository=$(IMG_REPOSITORY) \
		--set controller.image.tag=$(IMG_TAG) \
		--set controller.image.digest=$(IMG_DIGEST) \
		--set settings.preferencePolicy=Ignore \
		--set-string controller.env[0].name=ENABLE_PROFILING \
		--set-string controller.env[0].value=true

delete: ## Delete the controller from your ~/.kube/config cluster
	helm uninstall karpenter --namespace $(KARPENTER_NAMESPACE)

test: ## Run tests
	go test ./pkg/... \
		-race \
		-timeout 20m \
		--ginkgo.focus="${FOCUS}" \
		--ginkgo.randomize-all \
		--ginkgo.v \
		-cover -coverprofile=coverage.out -outputdir=. -coverpkg=./...

deflake: ## Run randomized, racing tests until the test fails to catch flakes
	ginkgo \
		--race \
		--focus="${FOCUS}" \
		--timeout=20m \
		--randomize-all \
		--until-it-fails \
		-v \
		./pkg/...

vulncheck: ## Verify code vulnerabilities
	@govulncheck ./pkg/...

licenses: download ## Verifies dependency licenses
	! go-licenses csv ./... | grep -v -e 'MIT' -e 'Apache-2.0' -e 'BSD-3-Clause' -e 'BSD-2-Clause' -e 'ISC' -e 'MPL-2.0'

verify: ## Verify code. Includes codegen, docgen, dependencies, linting, formatting, etc
	go mod tidy
	go generate ./...
	hack/validation/taint.sh
	hack/validation/requirements.sh
	hack/validation/labels.sh
	hack/validation/status.sh
	cp -r pkg/apis/crds kwok/charts
	hack/kwok/requirements.sh
	hack/dependabot.sh
	@# Use perl instead of sed due to https://stackoverflow.com/questions/4247068/sed-command-with-i-option-failing-on-mac-but-works-on-linux
	@# We need to do this "sed replace" until controller-tools fixes this parameterized types issue: https://github.com/kubernetes-sigs/controller-tools/issues/756
	@perl -i -pe 's/sets.Set/sets.Set[string]/g' pkg/scheduling/zz_generated.deepcopy.go
	hack/boilerplate.sh
	go vet ./...
	cd kwok/charts && helm-docs
	golangci-lint run
	@git diff --quiet ||\
		{ echo "New file modification detected in the Git working tree. Please check in before commit."; git --no-pager diff --name-only | uniq | awk '{print "  - " $$0}'; \
		if [ "${CI}" = true ]; then\
			exit 1;\
		fi;}
	actionlint -oneline

download: ## Recursively "go mod download" on all directories where go.mod exists
	$(foreach dir,$(MOD_DIRS),cd $(dir) && go mod download $(newline))

toolchain: ## Install developer toolchain
	./hack/toolchain.sh

gen_instance_types:
	go run kwok/tools/gen_instance_types.go > kwok/cloudprovider/instance_types.json

.PHONY: help presubmit install-kwok uninstall-kwok build apply delete test deflake vulncheck licenses verify download toolchain gen_instance_types
