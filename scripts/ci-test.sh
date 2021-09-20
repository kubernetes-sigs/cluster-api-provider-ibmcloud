#!/bin/bash

# Copyright 2018 The Kubernetes Authors.
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
# shellcheck source=../hack/ensure-go.sh
source "${REPO_ROOT}/hack/ensure-go.sh"

# Directory to store JUnit XML test report.
JUNIT_REPORT_DIR=${JUNIT_REPORT_DIR:-}

# If JUNIT_REPORT_DIR is unset, and ARTIFACTS is set, then have them match.
if [[ -z "${JUNIT_REPORT_DIR:-}" && -n "${ARTIFACTS:-}" ]]; then
  JUNIT_REPORT_DIR="${ARTIFACTS}"
fi

#Add Junit test reports to ARTIFACTS directory.
function addJunitFiles(){
  declare -a junitFileList
  readarray -t junitFileList <<< "$(find "${REPO_ROOT}" -name 'junit-*.xml')"

  for i in "${junitFileList[@]}"
  do 
    mv "${i}" "${ARTIFACTS}" 
  done
}

cd "${REPO_ROOT}" && \
	source ./scripts/fetch_ext_bins.sh && \
	fetch_tools && \
	setup_envs && \
	make generate lint test

addJunitFiles
