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

REGISTRY=gcr.io/k8s-staging-capi-ibmcloud
IMG=powervs-cloud-controller-manager

# POWERVS_CLOUD_CONTROLLER_COMMIT can be fetched from here https://github.com/openshift/cloud-provider-powervs/commits/main
POWERVS_CLOUD_CONTROLLER_COMMIT?=07d19bf
TAG?=$(POWERVS_CLOUD_CONTROLLER_COMMIT)

build-image-linux-amd64: init-buildx
	{                                                                   \
        set -e ;                                                            \
        docker buildx build \
                --build-arg TARGETPLATFORM=linux/amd64 --build-arg ARCH=amd64 \
                --build-arg POWERVS_CLOUD_CONTROLLER_COMMIT=$(POWERVS_CLOUD_CONTROLLER_COMMIT)\
                -t $(REGISTRY)/$(IMG):$(TAG)_linux_amd64 . --target centos-base; \
        }

build-image-linux-ppc64le: init-buildx
	{                                                                    \
        set -e ;                                                             \
        docker buildx build \
                --build-arg TARGETPLATFORM=linux/ppc64le --build-arg ARCH=ppc64le\
                --build-arg POWERVS_CLOUD_CONTROLLER_COMMIT=$(POWERVS_CLOUD_CONTROLLER_COMMIT)\
                -t $(REGISTRY)/$(IMG):$(TAG)_linux_ppc64le . --target centos-base; \
        }

build-image-and-push-linux-amd64: init-buildx
	{                                                                   \
	set -e ;                                                            \
	docker buildx build \
		--build-arg TARGETPLATFORM=linux/amd64 --build-arg ARCH=amd64 \
		--build-arg POWERVS_CLOUD_CONTROLLER_COMMIT=$(POWERVS_CLOUD_CONTROLLER_COMMIT)\
		-t $(REGISTRY)/$(IMG):$(TAG)_linux_amd64 . --push --target centos-base; \
	}

build-image-and-push-linux-ppc64le: init-buildx
	{                                                                    \
	set -e ;                                                             \
	docker buildx build \
		--build-arg TARGETPLATFORM=linux/ppc64le --build-arg ARCH=ppc64le\
		--build-arg POWERVS_CLOUD_CONTROLLER_COMMIT=$(POWERVS_CLOUD_CONTROLLER_COMMIT)\
		-t $(REGISTRY)/$(IMG):$(TAG)_linux_ppc64le . --push --target centos-base; \
	}

init-buildx:
	# Ensure we use a builder that can leverage it (the default on linux will not)
	-docker buildx rm multiarch-multiplatform-builder
	docker buildx create --use --name=multiarch-multiplatform-builder
	docker run --rm --privileged multiarch/qemu-user-static --reset --credential yes --persistent yes
	# Register gcloud as a Docker credential helper.
	# Required for "docker buildx build --push".
	gcloud auth configure-docker --quiet


build-and-push-multi-arch: build-image-and-push-linux-amd64 build-image-and-push-linux-ppc64le
	docker manifest create --amend $(REGISTRY)/$(IMG):$(TAG) $(REGISTRY)/$(IMG):$(TAG)_linux_amd64 $(REGISTRY)/$(IMG):$(TAG)_linux_ppc64le
	docker manifest push -p $(REGISTRY)/$(IMG):$(TAG)
