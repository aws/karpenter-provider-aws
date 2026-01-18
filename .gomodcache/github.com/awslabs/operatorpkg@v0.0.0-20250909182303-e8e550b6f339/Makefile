MOD_DIRS = $(shell find . -path "./website" -prune -o -name go.mod -type f -print | xargs dirname)

.PHONY: help
help: ## Display help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: presubmit
presubmit: verify test ## Run before submitting code

.PHONY: verify
verify: tidy ##
	$(foreach dir,$(MOD_DIRS),cd $(dir) && go generate ./... $(newline))
	$(foreach dir,$(MOD_DIRS),cd $(dir) && go vet ./... $(newline))
	$(foreach dir,$(MOD_DIRS),cd $(dir) && go fmt ./... $(newline))

.PHONY: tidy
tidy: ## Recursively "go mod tidy" on all directories where go.mod exists
	$(foreach dir,$(MOD_DIRS),cd $(dir) && go mod tidy $(newline))

.PHONY: test
test: ##
	$(foreach dir,$(MOD_DIRS),cd $(dir) && go test ./... $(newline))

define newline


endef