# Copyright 2021 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

.DEFAULT_GOAL:=help

ROOT_DIR_RELATIVE := ../..
include $(ROOT_DIR_RELATIVE)/common.mk

# Directories.
BIN_DIR := bin
SHARE_DIR := share

OS := $(shell go env GOOS)
RUST_TARGET := unknown-$(OS)-gnu

ifeq ($(OS), darwin)
	RUST_TARGET := apple-darwin
endif

$(BIN_DIR):
	mkdir -p $@

$(SHARE_DIR):
	mkdir -p $@

MDBOOK_EXTRACT_COMMAND := tar xfvz $(SHARE_DIR)/mdbook.tar.gz -C bin
MDBOOK_ARCHIVE_EXT := .tar.gz

ifeq ($(OS), windows)
	RUST_TARGET := pc-windows-msvc
	MDBOOK_ARCHIVE_EXT := .zip
	MDBOOK_EXTRACT_COMMAND := unzip -d /tmp
endif

help:  # Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[0-9A-Za-z_-]+:.*?##/ { printf "  \033[36m%-45s\033[0m %s\n", $$1, $$2 } /^\$$\([0-9A-Za-z_-]+\):.*?##/ { gsub("_","-", $$1); printf "  \033[36m%-45s\033[0m %s\n", tolower(substr($$1, 3, length($$1)-3)), $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

## --------------------------------------
## Hack / Tools
## --------------------------------------

##@ hack/tools:

GO_APIDIFF := $(BIN_DIR)/go-apidiff
$(GO_APIDIFF): $(BIN_DIR) go.mod go.sum ## Build a local copy of go-apidiff.
	go build -tags=tools -o $@ github.com/joelanford/go-apidiff

GOLANGCI_LINT := $(BIN_DIR)/golangci-lint
$(GOLANGCI_LINT): $(BIN_DIR) go.mod go.sum ## Build a local copy of golangci-lint.
	go build -tags=capibmtools -o $@ github.com/golangci/golangci-lint/cmd/golangci-lint

GOTESTSUM := $(BIN_DIR)/gotestsum
$(GOTESTSUM): $(BIN_DIR) go.mod go.sum
	go build -tags=tools -o $@ gotest.tools/gotestsum

KUSTOMIZE := $(BIN_DIR)/kustomize
$(KUSTOMIZE): $(BIN_DIR) go.mod go.sum ## Build a local copy of kustomize.
	CGO_ENABLED=0 go build -tags=tools -o $@ sigs.k8s.io/kustomize/kustomize/v4

MDBOOK_SHARE := $(SHARE_DIR)/mdbook$(MDBOOK_ARCHIVE_EXT)
$(MDBOOK_SHARE): ../../versions.mk $(SHARE_DIR)
	curl -sL -o $(MDBOOK_SHARE) "https://github.com/rust-lang/mdBook/releases/download/$(MDBOOK_VERSION)/mdBook-$(MDBOOK_VERSION)-x86_64-$(RUST_TARGET)$(MDBOOK_ARCHIVE_EXT)"

MDBOOK := $(BIN_DIR)/mdbook
$(MDBOOK): $(BIN_DIR) $(MDBOOK_SHARE)
	$(MDBOOK_EXTRACT_COMMAND)
	chmod +x $@
	touch -m $@

MDBOOK_EMBED := $(BIN_DIR)/mdbook-embed
$(MDBOOK_EMBED): $(BIN_DIR) go.mod go.sum
	go build -tags=tools -o $(BIN_DIR)/mdbook-embed sigs.k8s.io/cluster-api/hack/tools/mdbook/embed

MDBOOK_RELEASELINK := $(BIN_DIR)/mdbook-releaselink
$(MDBOOK_RELEASELINK): $(BIN_DIR) go.mod go.sum
	go build -tags=tools -o $(BIN_DIR)/mdbook-releaselink sigs.k8s.io/cluster-api/hack/tools/mdbook/releaselink

MDBOOK_TABULATE := $(BIN_DIR)/mdbook-tabulate
$(MDBOOK_TABULATE): $(BIN_DIR) go.mod go.sum
	go build -tags=tools -o $(BIN_DIR)/mdbook-tabulate sigs.k8s.io/cluster-api/hack/tools/mdbook/tabulate

GOJQ := $(BIN_DIR)/gojq
$(GOJQ): $(BIN_DIR) go.mod go.sum ## Build a local copy of gojq.
	go build -tags=tools -o $@ github.com/itchyny/gojq/cmd/gojq

CONVERSION_GEN := $(BIN_DIR)/conversion-gen
$(CONVERSION_GEN): $(BIN_DIR) go.mod go.sum ## Build a local copy of conversion-gen.
	go build -tags=tools -o $@ k8s.io/code-generator/cmd/conversion-gen

GINKGO := $(BIN_DIR)/ginkgo
$(GINKGO): $(BIN_DIR) go.mod go.sum ## Build a local copy of ginkgo.
	go build -tags=tools -o $@ github.com/onsi/ginkgo/v2/ginkgo

ENVSUBST := $(BIN_DIR)/envsubst
$(ENVSUBST): $(BIN_DIR) go.mod go.sum ## Build a local copy of envsubst.
	go build -tags=tools -o $@ github.com/drone/envsubst/v2/cmd/envsubst

MOCKGEN := $(BIN_DIR)/mockgen
$(MOCKGEN): $(BIN_DIR) go.mod go.sum ## Build a local copy of mockgen.
	go build -tags=tools -o $@ github.com/golang/mock/mockgen

CONTROLLER_GEN := $(BIN_DIR)/controller-gen
$(CONTROLLER_GEN): $(BIN_DIR) go.mod go.sum ## Build a local copy of controller-gen.
	go build -tags=tools -o $@ sigs.k8s.io/controller-tools/cmd/controller-gen

CONVERSION_VERIFIER := $(BIN_DIR)/conversion-verifier
$(CONVERSION_VERIFIER): $(BIN_DIR) go.mod go.sum ## Build a local copy of conversion-verifier.
	go build -tags=tools -o $@ sigs.k8s.io/cluster-api/hack/tools/conversion-verifier

SETUP_ENVTEST := $(BIN_DIR)/setup-envtest
$(SETUP_ENVTEST): $(BIN_DIR) go.mod go.sum ## Build a local copy of setup-envtest.
	go build -tags=tools -o $@ sigs.k8s.io/controller-runtime/tools/setup-envtest
