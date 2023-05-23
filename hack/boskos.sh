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

USER="cluster-api-provider-ibmcloud"

release_account(){
    url="http://${BOSKOS_HOST}/release?name=${BOSKOS_RESOURCE_NAME}&dest=dirty&owner=${USER}"
    status_code=$(curl -w '%{http_code}' -X POST ${url})

    if [[ ${status_code} != 200 ]]; then
        echo "Got invalid response- ${status_code}"
        exit 1
    fi 
}

checkout_account(){
    resource_type=$1
    url="http://${BOSKOS_HOST}/acquire?type=${resource_type}&state=free&dest=busy&owner=${USER}"
    output=$(curl -X POST ${url})
    [ $? = 0 ] && status_code=200

    if [[ ${status_code} == 200 ]]; then
        echo "export BOSKOS_RESOURCE_NAME=$(echo ${output} | jq -r '.name')"
        echo "export IBMCLOUD_API_KEY=$(echo ${output} | jq -r '.userdata["api-key"]')"
        echo "export BOSKOS_RESOURCE_GROUP=$(echo ${output} | jq -r '.userdata["resource-group"]')"
        echo "export BOSKOS_REGION=$(echo ${output} | jq -r '.userdata["region"]')"
        if [[ ${resource_type} == "powervs-service" ]]; then
            echo "export BOSKOS_RESOURCE_ID=$(echo ${output} | jq -r '.userdata["service-instance-id"]')"
             echo "export BOSKOS_ZONE=$(echo ${output} | jq -r '.userdata["zone"]')"
        fi
    else
        echo "Got invalid response- ${status_code}"
        exit 1
    fi
}

heartbeat_account(){
    count=0
    url="http://${BOSKOS_HOST}/update?name=${BOSKOS_RESOURCE_NAME}&state=busy&owner=${USER}"
    while [ ${count} -lt 120 ]
    do
        status_code=$(curl -s -o /dev/null -w '%{http_code}' -X POST ${url})
        if [[ ${status_code} != 200 ]]; then
            echo "Got invalid response - ${status_code}"
            exit 1
        fi
        count=$(( $count + 1 ))
        sleep 60
    done
}
