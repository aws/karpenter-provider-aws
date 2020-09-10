
# Image URL to use all building/pushing image targets
IMG ?= ${KO_DOCKER_REPO}/karpenter:latest
GOLINT_OPTIONS ?= "--set_exit_status=1"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: build test

# Run tests
test: fmt vet
	go test ./... -v -cover

# Build controller binary
build: fmt vet tidy
	go build -o bin/karpenter karpenter/main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: fmt vet
	go run karpenter/main.go --enable-leader-election=false --enable-webhook=false

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy:
	kustomize build config/dev | ko apply -B -f -

undeploy:
	kustomize build config/dev | ko delete -f -

# Run go fmt against code
fmt:
	golint $(GOLINT_OPTIONS) ./...
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Tidy up modules
tidy:
	go mod tidy

# Generate code. This is necessary if any API changes are made to ./pkg/apis
generate:
	./hack/update-codegen.sh

# Build the docker image
docker-build:
	docker build . -t ${IMG}

# Push the docker image
docker-push:
	docker push ${IMG}

docker-release: docker-build docker-push
