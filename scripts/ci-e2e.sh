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
NO_OF_RETRY=${NO_OF_RETRY:-"3"}

# shellcheck source=../hack/ensure-go.sh
source "${REPO_ROOT}/hack/ensure-go.sh"
# shellcheck source=../hack/ensure-kubectl.sh
source "${REPO_ROOT}/hack/ensure-kubectl.sh"
# shellcheck source=../hack/boskos.sh
source "${REPO_ROOT}/hack/boskos.sh"
# shellcheck source=../hack/kind-network-fix.sh
source "${REPO_ROOT}/hack/kind-network-fix.sh"

ARTIFACTS="${ARTIFACTS:-${PWD}/_artifacts}"
mkdir -p "${ARTIFACTS}/logs/"

ARCH=$(uname -m)
OS=$(uname -s)
IBMCLOUD_CLI_VERSION=${IBMCLOUD_CLI_VERSION:-"2.16.0"}
E2E_FLAVOR=${E2E_FLAVOR:-}
capibmadm=$(pwd)/bin/capibmadm

[ "${ARCH}" == "x86_64" ] && ARCH="amd64"

trap cleanup EXIT

cleanup(){
    # stop the boskos heartbeat
    [[ -z ${HEART_BEAT_PID:-} ]] || kill -9 "${HEART_BEAT_PID}" || true
}

retry() {
  cmd=$1
  for i in $(seq 1 "$NO_OF_RETRY"); do
    echo "Attempt: $i/$NO_OF_RETRY"
    ret_code=0
    $cmd || ret_code=$?
    if [ $ret_code = 0 ]; then
      break
    elif [ "$i" == "$NO_OF_RETRY" ]; then
      echo "All retry attempts failed!"
      exit $ret_code
    else
      sleep 1
    fi
  done
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

    ibmcloud config --check-version=false
    # Login to IBM Cloud using the API Key
    retry "ibmcloud login -a cloud.ibm.com --no-region"

    # Install power-iaas command-line plug-in and target the required service instance
    ibmcloud plugin install power-iaas -f
    CRN=$(ibmcloud resource service-instance ${IBMPOWERVS_SERVICE_INSTANCE_ID} --output json | jq -r '.[].crn')
    ibmcloud pi service-target ${CRN}

    # Create the network instance
    ${capibmadm} powervs network create --name ${IBMPOWERVS_NETWORK_NAME} --service-instance-id ${IBMPOWERVS_SERVICE_INSTANCE_ID} --zone ${ZONE}

}

init_network_powervs(){
    # Builds the capibmadm binary 
    make capibmadm

    create_powervs_network_instance

    # Creating PowerVS network port 
    ${capibmadm} powervs port create --network ${IBMPOWERVS_NETWORK_NAME} --description "capi-e2e" --service-instance-id ${IBMPOWERVS_SERVICE_INSTANCE_ID} --zone ${ZONE}

    # Get and assign the IPs to the required variables
    NEW_PORT=$(${capibmadm} powervs port list --service-instance-id ${IBMPOWERVS_SERVICE_INSTANCE_ID} --zone ${ZONE} --network ${IBMPOWERVS_NETWORK_NAME} -o json)
    no_of_ports=$(echo ${NEW_PORT} | jq '.items | length')
    if [[ ${no_of_ports} != 1 ]]; then
        echo "Failed to get the required number or ports, got - ${no_of_ports}"
        exit 1
    fi
    export IBMPOWERVS_VIP="$(echo ${NEW_PORT} | jq -r '.items[0].ipAddress')"
    export IBMPOWERVS_VIP_EXTERNAL="$(echo ${NEW_PORT} | jq -r '.items[0].externalIP')"
    export IBMPOWERVS_VIP_CIDR=${IBMPOWERVS_VIP_CIDR:="29"}
}

prerequisites_powervs(){
    # Assigning PowerVS variables
    export IBMPOWERVS_SSHKEY_NAME=${IBMPOWERVS_SSHKEY_NAME:-"powercloud-bot-key"}
    export IBMPOWERVS_IMAGE_NAME=${IBMPOWERVS_IMAGE_NAME:-"capibm-powervs-centos-streams8-1-27-2"}
    export IBMPOWERVS_SERVICE_INSTANCE_ID=${BOSKOS_RESOURCE_ID:-"d53da3bf-1f4a-42fa-9735-acf16b1a05cd"}
    export IBMPOWERVS_NETWORK_NAME="capi-net-$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | head --bytes 5)"
    export ZONE=${BOSKOS_ZONE:-"osa21"}
}

prerequisites_vpc(){
    # Assigning VPC variables
    export IBMVPC_REGION=${BOSKOS_REGION:-"jp-osa"}
    export IBMVPC_ZONE="${IBMVPC_REGION}-1"
    export IBMVPC_RESOURCEGROUP=${BOSKOS_RESOURCE_GROUP:-"fa5405a58226402f9a5818cb9b8a5a8a"}
    export IBMVPC_NAME=${BOSKOS_RESOURCE_NAME:-"capi-vpc-e2e"}
    export IBMVPC_IMAGE_NAME=${IBMVPC_IMAGE_NAME:-"capibm-vpc-ubuntu-2004-kube-v1-27-2"}
    export IBMVPC_PROFILE=${IBMVPC_PROFILE:-"bx2-4x16"}
    export IBMVPC_SSHKEY_NAME=${IBMVPC_SSHKEY_NAME:-"vpc-cloud-bot-key"}
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

    # Set common variables
    export DOCKER_BUILDKIT=1
    # Setting controller loglevel to allow debug logs from the VPC/PowerVS client
    export LOGLEVEL=5

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
