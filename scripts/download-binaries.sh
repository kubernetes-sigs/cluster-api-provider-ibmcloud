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

set -o errexit
set -o nounset
set -o pipefail

# Use DEBUG=1 ./scripts/download-binaries.sh to get debug output
curl_args="-Ls"

[[ -z "${DEBUG:-""}" ]] || {
    set -x
    curl_args="-L"
}

logEnd() {
    local msg='done'
    [ "$1" -eq 0 ] || msg='Err downloading assets'
    echo "$msg"
}

trap 'logEnd $?' EXIT

echo "About to download some binaries. This might take a while..."

root_dir="$(cd "$(dirname "$0")/.." ; pwd)"
dest_dir="${root_dir}/bin"
mkdir -p "${dest_dir}"

platform=$(uname -s|tr A-Z a-z)
kb_version="1.0.8"
kb_tgz="kubebuilder_${kb_version}_${platform}_amd64.tar.gz"
kb_url="https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${kb_version}/${kb_tgz}"
curl "${curl_args}O" "${kb_url}" \
      && tar xzfP "${kb_tgz}" -C "${dest_dir}" --strip-components=2 \
        && rm "${kb_tgz}"

ks_version="1.0.11"
ks_binary="kustomize_${ks_version}_${platform}_amd64"
ks_url="https://github.com/kubernetes-sigs/kustomize/releases/download/v${ks_version}/${ks_binary}"
curl "${curl_args}" "${ks_url}" "-o" "${dest_dir}/kustomize" \
  && chmod +x "${dest_dir}/kustomize"

echo    "# destination:"
echo    "#   ${dest_dir}"
echo    "# versions:"
echo -n "#   kustomize:      "; "${dest_dir}/kustomize" version
echo -n "#   etcd:           "; ("${dest_dir}/etcd" --version || :) | grep "etcd Version:"
echo -n "#   kube-apiserver: "; "${dest_dir}/kube-apiserver" --version
echo -n "#   kubectl:        "; "${dest_dir}/kubectl" version --client --short
echo -n "#   kubebuilder:    "; "${dest_dir}/kubebuilder" version
