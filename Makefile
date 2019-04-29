
# Image URL to use all building/pushing image targets
CONTROLLER_IMG ?= ibmcloud-cluster-api-controller
CLUSTERCTL_IMG ?= ibmcloud-cluster-api-clusterctl
VERSION ?= $(shell git describe --exact-match 2> /dev/null || \
                 git describe --match=$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)
REGISTRY ?= k8scloudprovider

PWD := $(shell pwd)
BASE_DIR := $(shell basename $(PWD))
# Keep an existing GOPATH, make a private one if it is undefined
GOPATH_DEFAULT := $(PWD)/.go
export GOPATH ?= $(GOPATH_DEFAULT)
GOBIN_DEFAULT := $(GOPATH)/bin
export GOBIN ?= $(GOBIN_DEFAULT)

# goang dep tools
DEP = github.com/golang/dep/cmd/dep
DEP_CHECK := $(shell command -v dep 2> /dev/null)

HAS_DEP := $(shell command -v dep;)

all: test manager

# Run tests
test: depend generate fmt vet manifests
	go test ./pkg/... ./cmd/... -coverprofile cover.out


# Build manager binary
build: manager clusterctl

manager: generate fmt vet
	go build -o bin/manager cmd/manager/main.go

clusterctl:
	go build -o bin/clusterctl cmd/clusterctl/main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: depend generate fmt vet
	go run ./cmd/manager/main.go

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	cat provider-components.yaml | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests:
	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go crd
	kustomize build config/default/ > provider-components.yaml
	echo "---" >> provider-components.yaml
	kustomize build vendor/sigs.k8s.io/cluster-api/config/default/ >> provider-components.yaml

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

# Generate code
generate:
ifndef GOPATH
	$(error GOPATH not defined, please define GOPATH. Run "go help gopath" to learn more about GOPATH)
endif
	go generate ./pkg/... ./cmd/...

# Build the docker image
docker-build: test
	docker build . -f cmd/manager/Dockerfile -t $(REGISTRY)/$(CONTROLLER_IMG):$(VERSION)
	docker build . -f cmd/clusterctl/Dockerfile -t $(REGISTRY)/$(CLUSTERCTL_IMG):$(VERSION)

# Push the docker image
docker-push:
	docker push $(REGISTRY)/$(CONTROLLER_IMG):$(VERSION)
	docker push $(REGISTRY)/$(CLUSTERCTL_IMG):$(VERSION)

quick-image:
	docker build . -f cmd/manager/Dockerfile -t $(REGISTRY)/$(CONTROLLER_IMG):$(VERSION)
	docker push $(REGISTRY)/$(CONTROLLER_IMG):$(VERSION)
	docker build . -f cmd/clusterctl/Dockerfile -t $(REGISTRY)/$(CLUSTERCTL_IMG):$(VERSION)
	docker push $(REGISTRY)/$(CLUSTERCTL_IMG):$(VERSION)

$(GOBIN):
	echo "create gobin"
	mkdir -p $(GOBIN)

work: $(GOBIN)

depend: work
ifndef HAS_DEP
	curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
endif
	dep ensure

clean:
	rm -f bin/manager bin/clusterctl
