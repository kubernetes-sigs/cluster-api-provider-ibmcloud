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

export DOCKER_CLI_EXPERIMENTAL=enabled

echo "Dump Docker Info"
docker info

echo "Docker version"
docker version

if ! docker buildx 2>&1 >/dev/null; then
  echo "buildx not available. Docker 19.03 or higher is required with experimental features enabled or buildx is not installed."
  exit 1
fi

# Ensure qemu is in binfmt_misc
# Docker desktop already has these in versions recent enough to have buildx
# We only need to do this setup on linux hosts
if [ "$(uname)" == 'Linux' ]; then
  docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
fi

# We can skip setup if the current builder already has multi-arch
# AND if it isn't the docker driver, which doesn't work
current_builder="$(docker buildx inspect)"
# linux/amd64, linux/arm64, linux/riscv64, linux/ppc64le, linux/s390x, linux/386, linux/arm/v7, linux/arm/v6
if ! grep -q "^Driver: docker$"  <<<"${current_builder}" && \
     grep -q "linux/amd64" <<<"${current_builder}" && \
     grep -q "linux/arm"   <<<"${current_builder}" && \
     grep -q "linux/arm64" <<<"${current_builder}" && \
     grep -q "linux/s390x" <<<"${current_builder}"; then
  exit 0
fi

# Ensure we use a builder that can leverage it (the default on linux will not)
docker buildx rm capibm || true
docker buildx create --use --name=capibm
