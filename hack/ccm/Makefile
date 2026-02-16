# Copyright 2022 The Kubernetes Authors.
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

# Go
GO_VERSION ?=1.24.13
GO_CONTAINER_IMAGE ?= golang:$(GO_VERSION)

REGISTRY=gcr.io/k8s-staging-capi-ibmcloud
IMG=powervs-cloud-controller-manager

# POWERVS_CLOUD_CONTROLLER_COMMIT can be fetched from here https://github.com/openshift/cloud-provider-powervs/commits/main
POWERVS_CLOUD_CONTROLLER_COMMIT?=5ea9317
TAG?=$(POWERVS_CLOUD_CONTROLLER_COMMIT)

build-image: init-buildx gcloud-auth
	{ \
		set -e ; \
		docker buildx build \
			--platform=linux/amd64,linux/ppc64le --pull \
			--build-arg golang_image=$(GO_CONTAINER_IMAGE) \
			--build-arg POWERVS_CLOUD_CONTROLLER_COMMIT=$(POWERVS_CLOUD_CONTROLLER_COMMIT) \
			-t $(REGISTRY)/$(IMG):$(TAG) . --push --target centos-base; \
    }

init-buildx:
	# Ensure we use a builder that can leverage it (the default on linux will not)
	-docker buildx rm multiarch-multiplatform-builder
	docker buildx create --use --name=multiarch-multiplatform-builder
	docker run --rm --privileged multiarch/qemu-user-static --reset --credential yes --persistent yes

gcloud-auth:
	# Register gcloud as a Docker credential helper.
	# Required for "docker buildx build --push".
	gcloud auth configure-docker --quiet

build-local: init-buildx
	{ \
		set -e ; \
		docker buildx build \
			--platform=linux/$(ARCH) --pull \
			--build-arg golang_image=$(GO_CONTAINER_IMAGE) \
			--build-arg POWERVS_CLOUD_CONTROLLER_COMMIT=$(POWERVS_CLOUD_CONTROLLER_COMMIT) \
			-t $(REGISTRY)/$(IMG)-$(ARCH):$(TAG) . --output=type=docker --target centos-base; \
    }
