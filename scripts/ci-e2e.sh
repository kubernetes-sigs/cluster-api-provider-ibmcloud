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
GOPATH_BIN="$(go env GOPATH)/bin/"
export PATH="${GOPATH_BIN}:${PATH}"
RESOURCE_TYPE="${RESOURCE_TYPE:-"powervs-service"}"

# shellcheck source=../hack/ensure-go.sh
source "${REPO_ROOT}/hack/ensure-go.sh"
# shellcheck source=../hack/ensure-kubectl.sh
source "${REPO_ROOT}/hack/ensure-kubectl.sh"
# shellcheck source=../hack/boskos.sh
source ${REPO_ROOT}/hack/boskos.sh

ARTIFACTS="${ARTIFACTS:-${PWD}/_artifacts}"
mkdir -p "${ARTIFACTS}/logs/"

ARCH=$(uname -m)
OS=$(uname -s)
IBMCLOUD_CLI_VERSION=${IBMCLOUD_CLI_VERSION:-"2.13.0"}
PVSADM_VERSION=${PVSADM_VERSION:-"v0.1.9"}
E2E_FLAVOR=${E2E_FLAVOR:-}
REGION=${REGION:-"us-south"}

[ "${ARCH}" == "x86_64" ] && ARCH="amd64"

trap cleanup EXIT

cleanup(){
    # stop the boskos heartbeat
    [[ -z ${HEART_BEAT_PID:-} ]] || kill -9 "${HEART_BEAT_PID}" || true
}

# Installing binaries from github releases
install_pvsadm(){
    platform="$(echo ${OS} | tr '[:upper:]' '[:lower:]')-${ARCH}"

    curl -fsL https://github.com/ppc64le-cloud/pvsadm/releases/download/${PVSADM_VERSION}/pvsadm-${platform} -o pvsadm
    chmod +x ./pvsadm
}

install_ibmcloud_cli(){
    if [ ${OS} == "Linux" ]; then
        platform="linux_${ARCH}"
    elif [ ${OS} == "Darwin" ]; then
        platform="macos"
    fi
     
    curl https://download.clis.cloud.ibm.com/ibm-cloud-cli/${IBMCLOUD_CLI_VERSION}/binaries/IBM_Cloud_CLI_${IBMCLOUD_CLI_VERSION}_${platform}.tgz -o IBM_Cloud_CLI_${IBMCLOUD_CLI_VERSION}_${platform}.tgz
    tar -xf IBM_Cloud_CLI_${IBMCLOUD_CLI_VERSION}_${platform}.tgz
    install IBM_Cloud_CLI/ibmcloud /usr/local/bin 

}

create_powervs_network_instance(){
    install_ibmcloud_cli

    # Login to IBM Cloud using the API Key
    ibmcloud login -a cloud.ibm.com -r ${REGION}

    # Install power-iaas command-line plug-in and target the required service instance
    ibmcloud plugin install power-iaas
    CRN=$(ibmcloud resource service-instance ${IBMPOWERVS_SERVICE_INSTANCE_ID} --output json | jq -r '.[].crn')
    ibmcloud pi service-target ${CRN}

    # Create the network instance
    ibmcloud pi network-create-public ${IBMPOWERVS_NETWORK_NAME} --dns-servers "8.8.8.8 9.9.9.9"

}

init_network_powervs(){
    install_pvsadm
    create_powervs_network_instance

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
    export IBMPOWERVS_SSHKEY_NAME=${IBMPOWERVS_SSHKEY_NAME:-"powercloud-bot-key"}
    export IBMPOWERVS_IMAGE_NAME=${IBMPOWERVS_IMAGE_NAME:-"capibm-powervs-centos-streams8-1-25-1"}
    export IBMPOWERVS_SERVICE_INSTANCE_ID=${BOSKOS_RESOURCE_ID:-"d53da3bf-1f4a-42fa-9735-acf16b1a05cd"}
    export IBMPOWERVS_NETWORK_NAME="capi-net-$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | head --bytes 5)"
    # Setting controller loglevel to allow debug logs from the PowerVS client
    export LOGLEVEL=5
}

prerequisites_vpc(){
    # Assigning VPC variables
    export IBMVPC_REGION=${BOSKOS_REGION:-"jp-osa"}
    export IBMVPC_ZONE="${IBMVPC_REGION}-1"
    export IBMVPC_RESOURCEGROUP=${BOSKOS_RESOURCE_GROUP:-"fa5405a58226402f9a5818cb9b8a5a8a"}
    export IBMVPC_NAME=${BOSKOS_RESOURCE_NAME:-"capi-vpc-e2e"}
    export IBMVPC_IMAGE_NAME=${IBMVPC_IMAGE_NAME:-"capibm-vpc-ubuntu-2004-kube-v1-25-2"}
    export IBMVPC_PROFILE=${IBMVPC_PROFILE:-"bx2-4x16"}
    export IBMVPC_SSHKEY_NAME=${IBMVPC_SSHKEY_NAME:-"vpc-cloud-bot-key"}
    # Setting controller loglevel to allow debug logs from the VPC client
    export LOGLEVEL=5
}

prerequisites_vpc_load_balancer(){
    # Assigning VPC LoadBalancer variables
    export PROVIDER_ID_FORMAT=v2
    export EXP_CLUSTER_RESOURCE_SET=true
    export IBMACCOUNT_ID=${IBMACCOUNT_ID:-"7cfbd5381a434af7a09289e795840d4e"}
    export BASE64_API_KEY=$(echo -n $IBMCLOUD_API_KEY | base64)
}

main(){

    [[ "${E2E_FLAVOR}" == "vpc"* ]] && RESOURCE_TYPE="vpc-service"

    # If BOSKOS_HOST is set then acquire an IBM Cloud resource from Boskos.
    if [ -n "${BOSKOS_HOST:-}" ]; then
        # Check out the resource from Boskos and store the produced environment
        # variables in a temporary file.
         account_env_var_file="$(mktemp)"
         checkout_account ${RESOURCE_TYPE} 1> "${account_env_var_file}"
         checkout_account_status="${?}"

        # If the checkout process was a success then load the
        # environment variables into this process.
        [ "${checkout_account_status}" = "0" ] && . "${account_env_var_file}"

        # Always remove the account environment variable file which
        # could contain sensitive information.
        rm -f "${account_env_var_file}"

        if [ ! "${checkout_account_status}" = "0" ]; then
            echo "error getting account from boskos" 1>&2
            exit "${checkout_account_status}"
        fi

        # Run the heart beat process to tell Boskos that we are still
        # using the checked out resource periodically.
        heartbeat_account >> "$ARTIFACTS/logs/boskos.log" 2>&1 &
        HEART_BEAT_PID=$(echo $!)
    fi

    if [[ "${E2E_FLAVOR}" == "powervs" || "${E2E_FLAVOR}" == "powervs-md-remediation" ]]; then
        prerequisites_powervs
        init_network_powervs
    fi

    if [[ "${E2E_FLAVOR}" == "vpc"* ]]; then
        prerequisites_vpc
    fi

    if [[ "${E2E_FLAVOR}" == "vpc-load-balancer" ]]; then
        prerequisites_vpc_load_balancer
    fi

    # Run the e2e tests
    make test-e2e E2E_FLAVOR=${E2E_FLAVOR}
    test_status="${?}"
    echo TESTSTATUS="${test_status}"

    # If Boskos is being used then release the IBM Cloud resource back to Boskos.
    [ -z "${BOSKOS_HOST:-}" ] || release_account >> "$ARTIFACTS/logs/boskos.log" 2>&1
}

main "$@"
exit "${test_status}"
