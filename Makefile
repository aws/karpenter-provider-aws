DOCKER_REPO ?= ${KO_DOCKER_REPO}
IMAGE_NAME = karpenter
TAG ?= latest
IMAGE_URI ?= ${DOCKER_REPO}/${IMAGE_NAME}:${TAG}

SOURCE_PATHS=./pkg/... ./cmd/...

run:
	go run ./cmd/karpenter/main.go

release: build test push

build:
	go vet ${SOURCE_PATHS}
	go fmt ${SOURCE_PATHS}
	docker build . --tag ${IMAGE_URI}

test:
	go test ${SOURCE_PATHS} -coverprofile cover.out

push:
	docker push ${IMAGE_URI}