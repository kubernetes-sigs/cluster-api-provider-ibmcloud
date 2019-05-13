#!/usr/bin/env bash
# Copyright 2019 The Kubernetes Authors.
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

# This script validates that binaries can be built and that all tests pass.

set -o errexit
set -o nounset
set -o pipefail


base_dir="$(cd "$(dirname "$0")/.." ; pwd)"

TAG=${TAG:-}
BRANCH=${BRANCH:-master}
echo "tag: ${TAG}"
echo "branch:${BRANCH}"
if [[ "${TAG}" =~ ^v([0-9]\.)+([0-9])[-a-zA-Z0-9]*([.0-9])* ]]; then
   echo "Using tag: '${TAG}' and 'latest' ."
    TAG="${TAG}"
    LATEST="latest"
elif [[ "${BRANCH}" == "master" ]]; then
   echo "Using tag: 'canary'."
    TAG="canary"
    LATEST=""
else
    echo "Nothing to deploy. Image build skipped." >&2
    exit 0
fi

echo "Starting image build"
export REGISTRY=quay.io/cluster-api-provider-ibmcloud

echo "Building controller and clusterctl docker images."
cd ${base_dir}
VERSION=$TAG make images
cd -

echo "Logging into registry ${REGISTRY%%/*}"
docker login -u "${REGISTRY_USERNAME}" -p "${REGISTRY_PASSWORD}"

echo "Pushing images with tag '${TAG}'."
VERSION=$TAG make push-images

if [ "$LATEST" == "latest" ]; then
   docker tag ${REGISTRY}/controller:${TAG} ${REGISTRY}/controller:${LATEST}
   docker tag ${REGISTRY}/clusterctl:${TAG} ${REGISTRY}/clusterctl:${LATEST}
   echo "Pushing images with tag '${LATEST}'."
   docker push ${REGISTRY}/controller:${LATEST}
   docker push ${REGISTRY}/clusterctl:${LATEST}
fi
