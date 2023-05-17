#!/usr/bin/env bash

# Copyright 2023 The Kubernetes Authors.
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

if [ -n "$DEBUG" ]; then
	set -x
fi

set -o errexit
set -o nounset
set -o pipefail

if ! docker buildx 2>&1 >/dev/null; then
  echo "buildx not available. Ensure buildx is installed and Docker version >= 19.03 is installed with experimental features enabled."
  exit 1
fi

# We can skip setup if the current builder already has multi-arch
# AND if it isn't the docker driver, which doesn't work
current_builder="$(docker buildx inspect)"
# linux/amd64, linux/arm64, linux/riscv64, linux/ppc64le, linux/s390x, linux/386, linux/arm/v7, linux/arm/v6
if ! grep -q "^Driver:[[:space:]]*docker$" <<<"${current_builder}" && \
     grep -q "linux/amd64" <<<"${current_builder}" && \
     grep -q "linux/arm64" <<<"${current_builder}" && \
     grep -q "linux/ppc64le" <<<"${current_builder}"; then
  exit 0
fi

# Ensure qemu is in binfmt_misc
# Docker desktop already has these in versions recent enough to have buildx
# We only need to do this setup on linux hosts
if [ "$(uname)" == 'Linux' ]; then
  # NOTE: this is pinned to a digest for a reason!
  docker run --rm --privileged tonistiigi/binfmt:qemu-v7.0.0-28@sha256:66e11bea77a5ea9d6f0fe79b57cd2b189b5d15b93a2bdb925be22949232e4e55 --install all
fi

# Ensure we use a builder that can leverage it (the default on linux will not)
docker buildx rm capibm >/dev/null 2>&1 || true
docker buildx create --use --name=capibm
