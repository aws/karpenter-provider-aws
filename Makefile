# Image URL to use all building/pushing image targets
IMG ?= ${KO_DOCKER_REPO}/karpenter:latest

all: verify generate build test

# Run tests
test:
	go test ./... -v -cover

# Build controller binary
build:
	go build -o bin/karpenter karpenter/main.go

# Build and release a container image
release:
	docker build . -t ${IMG}
	docker push ${IMG}

# Run against the configured Kubernetes cluster in ~/.kube/config
run: verify
	go run karpenter/main.go --enable-leader-election=false --enable-webhook=false

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config

deploy:
	kustomize build config/dev | ko apply -B -f -

undeploy:
	kustomize build config/dev | ko delete -f -

# Verify code. Includes dependencies, linting, formatting, etc
verify:
	go mod tidy
	go mod download
	go vet ./...
	go fmt ./...
	golangci-lint run

# Generate code. Must be run if changes are made to ./pkg/apis/...
generate:
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
