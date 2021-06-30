// +build tools

package tools

import (
	_ "github.com/ahmetb/gen-crd-api-reference-docs"
	_ "github.com/fzipp/gocyclo"
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/google/ko/cmd/ko"
	_ "github.com/mikefarah/yq/v4"
	_ "github.com/mitchellh/golicense"
	_ "github.com/onsi/ginkgo/ginkgo"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
