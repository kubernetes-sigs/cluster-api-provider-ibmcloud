# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Build the cluster binary
FROM golang:1.12.5 as builder

# Copy in the go src
WORKDIR /go/src/sigs.k8s.io/cluster-api-provider-ibmcloud
COPY pkg/    pkg/
COPY cmd/    cmd/
COPY vendor/ vendor/

# build clusterctl
RUN CGO_ENABLED=0 GOOS=linux go build -a -o clusterctl sigs.k8s.io/cluster-api-provider-ibmcloud/cmd/clusterctl

# Copy clusterctl into a thin image
FROM alpine:latest

RUN apk add --no-cache ca-certificates openssh-client curl \
    && curl -L https://download.docker.com/linux/static/stable/x86_64/docker-17.09.1-ce.tgz | tar --strip-components=1 -xvz -C /bin/ docker/docker \
    && curl -L -o /bin/kind https://github.com/kubernetes-sigs/kind/releases/download/v0.3.0/kind-linux-amd64 \
    && curl -L -o /bin/kubectl https://storage.googleapis.com/kubernetes-release/release/v1.14.2/bin/linux/amd64/kubectl \
    && chmod +x /bin/kubectl /bin/kind

COPY --from=builder /go/src/sigs.k8s.io/cluster-api-provider-ibmcloud/clusterctl /bin/

ENTRYPOINT ["clusterctl"]
