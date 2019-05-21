GIT_HOST = sigs.k8s.io
PWD := $(shell pwd)
BASE_DIR := $(shell basename $(PWD))
# Keep an existing GOPATH, make a private one if it is undefined
GOPATH_DEFAULT := $(PWD)/.go
export GOPATH ?= $(GOPATH_DEFAULT)
GOBIN_DEFAULT := $(GOPATH)/bin
export GOBIN ?= $(GOBIN_DEFAULT)
TESTARGS_DEFAULT := "-v"
export TESTARGS ?= $(TESTARGS_DEFAULT)
PKG := $(shell awk  -F "\"" '/^ignored = / { print $$2 }' Gopkg.toml)
DEST := $(GOPATH)/src/$(GIT_HOST)/$(BASE_DIR)
SOURCES := $(shell find $(DEST) -name '*.go')

HAS_DEP := $(shell command -v dep;)
HAS_LINT := $(shell command -v golint;)
HAS_KUSTOMIZE := $(shell command -v kustomize;)
GOX_PARALLEL ?= 3
TARGETS ?= darwin/amd64 linux/amd64 linux/386 linux/arm linux/arm64 linux/ppc64le
DIST_DIRS         = find * -type d -exec

GENERATE_YAML_PATH=cmd/clusterctl/examples/ibmcloud
GENERATE_YAML_EXEC=generate-yaml.sh
GENERATE_YAML_TEST_FOLDER=dummy-make-auto-test

GOOS ?= $(shell go env GOOS)
VERSION ?= $(shell git describe --exact-match 2> /dev/null || \
                 git describe --match=$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)
GOFLAGS   :=
TAGS      :=
LDFLAGS   := "-w -s -X 'main.version=${VERSION}'"

# Image URL to use all building/pushing image targets
CONTROLLER_IMG ?= controller
CLUSTERCTL_IMG ?= clusterctl
VERSION ?= $(shell git describe --exact-match 2> /dev/null || \
                 git describe --match=$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)
REGISTRY ?= quay.io/cluster-api-provider-ibmcloud

ifneq ("$(realpath $(DEST))", "$(realpath $(PWD))")
	$(error Please run 'make' from $(DEST). Current directory is $(PWD))
endif

all: test build images

############################################################
# depend section
############################################################
$(GOBIN):
	echo "create gobin"
	mkdir -p $(GOBIN)

work: $(GOBIN)

kubebuilder:
	echo "checking if kubebuilder exists or not"
	if [ ! -d "/usr/local/kubebuilder" ]; then \
		curl -LO https://github.com/kubernetes-sigs/kubebuilder/releases/download/v1.0.8/kubebuilder_1.0.8_linux_amd64.tar.gz \
		&& tar xzf kubebuilder_1.0.8_linux_amd64.tar.gz \
		&& mv kubebuilder_1.0.8_linux_amd64 kubebuilder && mv kubebuilder /usr/local/ \
		&& rm kubebuilder_1.0.8_linux_amd64.tar.gz; \
	fi	

depend: work kubebuilder
ifndef HAS_DEP
	curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
endif
	dep ensure

depend-update: work
	dep ensure -update

############################################################
# generate section
############################################################
generate: manifests
ifndef GOPATH
	$(error GOPATH not defined, please define GOPATH. Run "go help gopath" to learn more about GOPATH)
endif
	go generate ./pkg/... ./cmd/...

manifests:
	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go crd

############################################################
# check section
############################################################
check: fmt vet lint

fmt: depend generate
	hack/verify-gofmt.sh

lint: depend generate
ifndef HAS_LINT
		go get -u golang.org/x/lint/golint
		echo "installing golint"
endif
	hack/verify-golint.sh

vet: depend generate
	go vet ./...

############################################################
# test section
############################################################
test: unit functional fmt vet generate_yaml_test

unit: depend generate check
	go test -tags=unit $(shell go list ./...) $(TESTARGS)

functional:
	@echo "$@ not yet implemented"

# Generate manifests e.g. CRD, RBAC etc.
generate_yaml_test:
ifndef HAS_KUSTOMIZE
	curl -LO https://github.com/kubernetes-sigs/kustomize/releases/download/v2.0.3/kustomize_2.0.3_linux_amd64
	mv kustomize_2.0.3_linux_amd64 /usr/local/bin/kustomize
	chmod +x /usr/local/bin/kustomize
endif
	# Create a dummy file for test only
	echo 'clouds' > cmd/clusterctl/examples/ibmcloud/dummy-clouds-test.yaml
	$(GENERATE_YAML_PATH)/$(GENERATE_YAML_EXEC) -f dummy-clouds-test.yaml ubuntu $(GENERATE_YAML_TEST_FOLDER)
	# the folder will be generated under same folder of $(GENERATE_YAML_PATH)
	rm -fr $(GENERATE_YAML_PATH)/$(GENERATE_YAML_TEST_FOLDER)
	rm -f cmd/clusterctl/examples/ibmcloud/dummy-clouds-test.yaml

############################################################
# build section
############################################################
build: manager clusterctl

manager: depend generate check
	CGO_ENABLED=0 GOOS=$(GOOS) go build \
		-ldflags $(LDFLAGS) \
		-o bin/manager \
		cmd/manager/main.go

clusterctl: depend generate check
	CGO_ENABLED=0 GOOS=$(GOOS) go build \
		-ldflags $(LDFLAGS) \
		-o bin/clusterctl \
		cmd/clusterctl/main.go

############################################################
# deploy section
############################################################
# Run against the configured Kubernetes cluster in ~/.kube/config
run: depend generate check
	go run ./cmd/manager/main.go

# Install CRDs into a cluster
install: generate_yaml_test
	kubectl apply -f config/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: generate_yaml_test
	cat provider-components.yaml | kubectl apply -f -

############################################################
# images section
############################################################
# Build the docker image
clusterctl-image: manifests
	docker build . -f cmd/clusterctl/Dockerfile -t $(REGISTRY)/$(CLUSTERCTL_IMG):$(VERSION)
controller-image: manifests
	docker build . -f cmd/manager/Dockerfile -t $(REGISTRY)/$(CONTROLLER_IMG):$(VERSION)

push-clusterctl-image:
	docker push $(REGISTRY)/$(CLUSTERCTL_IMG):$(VERSION)
push-controller-image:
	docker push $(REGISTRY)/$(CONTROLLER_IMG):$(VERSION)

images: test clusterctl-image controller-image
push-images: push-clusterctl-image push-controller-image

build-push-images: images push-images

# quickly get target image
new-controller: controller-image push-controller-image
new-clusterctl: clusterctl-image push-clusterctl-image

############################################################
# clean section
############################################################
clean:
	rm -f bin/manager bin/clusterctl

realclean: clean
	rm -rf vendor
	if [ "$(GOPATH)" = "$(GOPATH_DEFAULT)" ]; then \
		rm -rf $(GOPATH); \
	fi
