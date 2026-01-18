# Copyright 2018 The Prometheus Authors
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

include Makefile.common

BUF := $(FIRST_GOPATH)/bin/buf
BUF_VERSION ?= v1.39.0

$(BUF):
	go install github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)

.PHONY: deps
deps:
	$(MAKE) common-deps
	cd exp && $(GO) mod tidy && $(GO) mod download

.PHONY: test
test: deps common-test test-exp

.PHONY: test-short
test-short: deps common-test-short test-exp-short

.PHONY: generate-go-collector-test-files
file := supported_go_versions.txt
VERSIONS := $(shell cat ${file})
generate-go-collector-test-files:
	for GO_VERSION in $(VERSIONS); do \
		docker run \
			--platform linux/amd64 \
			--rm -v $(PWD):/workspace \
			-w /workspace \
			golang:$$GO_VERSION \
			bash ./generate-go-collector.bash; \
	done; \
	go mod tidy

.PHONY: fmt
fmt: common-format

.PHONY: proto
proto: ## Regenerate Go from remote write proto.
proto: $(BUF)
	@echo ">> regenerating Prometheus Remote Write proto"
	@cd exp/api/remote/genproto && $(BUF) generate
	@cd exp/api/remote && find genproto/ -type f -exec sed -i '' 's/protohelpers "github.com\/planetscale\/vtprotobuf\/protohelpers"/protohelpers "github.com\/prometheus\/client_golang\/exp\/internal\/github.com\/planetscale\/vtprotobuf\/protohelpers"/g' {} \;
	# For some reasons buf generates this unused import, kill it manually for now and reformat.
	@cd exp/api/remote && find genproto/ -type f -exec sed -i '' 's/_ "github.com\/gogo\/protobuf\/gogoproto"//g' {} \;
	@cd exp/api/remote && go fmt ./genproto/...
	$(MAKE) fmt

.PHONY: test-exp
test-exp:
	cd exp && $(GOTEST) $(test-flags) $(GOOPTS) $(pkgs)

.PHONY: test-exp-short
test-exp-short:
	cd exp && $(GOTEST) -short $(GOOPTS) $(pkgs)
