#!/bin/bash

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

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd "${REPO_ROOT}" || exit 1

# shellcheck source=../hack/ensure-go.sh
source "${REPO_ROOT}/hack/ensure-go.sh"

ARTIFACTS="${ARTIFACTS:-${PWD}/_artifacts}"
mkdir -p "${ARTIFACTS}/logs/"

ARCH=$(uname -m)
PVSADM_VERSION=${PVSADM_VERSION:-"v0.1.3"}
E2E_FLAVOR=${E2E_FLAVOR:-}

trap cleanup EXIT

cleanup(){
    # Delete the created ports for the network instance
    [ -n "${NEW_PORT}" ] && ./pvsadm delete port --network ${IBMPOWERVS_NETWORK_NAME} --port-id ${PORT_ID} --instance-id ${IBMPOWERVS_SERVICE_INSTANCE_ID}
}

install_pvsadm(){
    [ "${ARCH}" == "x86_64" ] && ARCH="amd64"

    # Installing binaries from github releases
    curl -fsL https://github.com/ppc64le-cloud/pvsadm/releases/download/${PVSADM_VERSION}/pvsadm-linux-${ARCH} -o pvsadm
    chmod +x ./pvsadm
}

init_network_powervs(){
    install_pvsadm

    # Creating ports using the pvsadm tool
    ./pvsadm create port --description "capi-port-e2e" --network ${IBMPOWERVS_NETWORK_NAME} --instance-id ${IBMPOWERVS_SERVICE_INSTANCE_ID}

    # Get and assign the IPs to the required variables
    NEW_PORT=$(./pvsadm get ports --network ${IBMPOWERVS_NETWORK_NAME} --instance-id ${IBMPOWERVS_SERVICE_INSTANCE_ID} | sed -n '4 p')
    PORT_ID="$(echo ${NEW_PORT} | cut -d'|' -f6 | xargs )"
    export IBMPOWERVS_VIP="$(echo ${NEW_PORT} | cut -d'|' -f4 | xargs )"
    export IBMPOWERVS_VIP_EXTERNAL="$(echo ${NEW_PORT} | cut -d'|' -f3 | xargs )"
    export IBMPOWERVS_VIP_CIDR=${IBMPOWERVS_VIP_CIDR:="29"}
}

prerequisites_powervs(){
    # Assigning PowerVS variables
    export IBMPOWERVS_IMAGE_NAME=${IBMPOWERVS_IMAGE_NAME:-"capibm-powervs-centos-streams8-1-22-4"}
    export IBMPOWERVS_SERVICE_INSTANCE_ID=${IBMPOWERVS_SERVICE_INSTANCE_ID:-"0f28d13a-6e33-4d86-b6d7-a9b46ff7659e"}
    export IBMPOWERVS_NETWORK_NAME=${IBMPOWERVS_NETWORK_NAME:-"capi-e2e-test"}
}

main(){
    if [[ "${E2E_FLAVOR}" == "powervs" ]]; then
        prerequisites_powervs
        init_network_powervs
    fi

    # Run the e2e tests
    make test-e2e
    test_status="${?}"
    echo TESTSTATUS="${test_status}"
}

main "$@"
exit "${test_status}"
