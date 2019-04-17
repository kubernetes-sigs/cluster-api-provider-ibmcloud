#!/usr/bin/env bash
# Copyright 2017 The Kubernetes Authors.
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

source "$(dirname "${BASH_SOURCE}")/util.sh"
ROOT_DIR="$(cd "$(dirname "$0")/.." ; pwd)"
MAKE_CMD="make -C ${ROOT_DIR}"
NUM_CLUSTERS="${NUM_CLUSTERS:-2}"
JOIN_CLUSTERS="${JOIN_CLUSTERS:-}"
DOWNLOAD_BINARIES="${DOWNLOAD_BINARIES:-}"
CONTAINER_REGISTRY_HOST="${CONTAINER_REGISTRY_HOST:-172.17.0.1:5000}"

function build-binaries() {
  ${MAKE_CMD} manager
}

function run-unit-tests() {
  ${MAKE_CMD} test
}

function check-make-generate-output() {
  ${MAKE_CMD} generate
  echo "Checking state of working tree after running 'make generate'"
  check-git-state
}

function check-git-state() {
  local output
  if output=$(git status --porcelain) && [ -z "${output}" ]; then
    return
  fi
  echo "ERROR: the working tree is dirty:"
  for line in "${output}"; do
    echo "${line}"
  done
  git diff
  return 1
}

# Make sure, we run in the root of the repo and
# therefore run the tests on all packages
cd "$ROOT_DIR" || {
  echo "Cannot cd to '$ROOT_DIR'. Aborting." >&2
  exit 1
}

export PATH=${ROOT_DIR}/bin:${PATH}

echo "Checking initial state of working tree"
check-git-state

echo "Verifying Gofmt"
./hack/go-tools/verify-gofmt.sh

echo "Checking that correct Error Package is used."
./hack/verify-errpkg.sh

echo "Checking that 'make generate' is up-to-date"
check-make-generate-output

echo "Building federation binaries"
build-binaries

echo "Running unit tests"
run-unit-tests
