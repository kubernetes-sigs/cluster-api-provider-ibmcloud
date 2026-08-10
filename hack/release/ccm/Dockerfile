# syntax=docker/dockerfile:1.5

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

# Build IBM cloud controller manager binary
ARG golang_image
FROM --platform=$BUILDPLATFORM ${golang_image} AS ccm-builder
ARG TARGETARCH
ARG POWERVS_CLOUD_CONTROLLER_COMMIT
WORKDIR /build
RUN git clone https://github.com/openshift/cloud-provider-powervs
RUN cd cloud-provider-powervs && git checkout $POWERVS_CLOUD_CONTROLLER_COMMIT && CGO_ENABLED=0 GOARCH=$TARGETARCH go build \
     -ldflags "-s -w" -o /build/ibm-cloud-controller-manager .

# Assemble the final image
FROM --platform=$TARGETPLATFORM quay.io/centos/centos:stream8 AS centos-base
LABEL description="IBM PowerVS Cloud Controller Manager"
COPY --from=ccm-builder /build/ibm-cloud-controller-manager /bin/ibm-cloud-controller-manager
ENTRYPOINT [ "/bin/ibm-cloud-controller-manager" ]
