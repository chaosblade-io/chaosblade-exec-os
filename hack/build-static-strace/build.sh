#!/usr/bin/env bash
# Copyright 2025 The ChaosBlade Authors
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

# Build the bundled static strace for linux/arm64 and drop it into
# extra/strace/linux_arm64/strace, ready for `make linux_arm64`.
#
# Only arm64 is built here: the amd64 binary (extra/strace/linux_amd64/strace)
# is the pre-existing, production-validated one and is intentionally NOT
# rebuilt. See Dockerfile for the full rationale.
#
# Requires: docker with buildx. The artifact is consumed by the Makefile, which
# copies extra/strace/<platform>/strace next to chaos_os.
#
# Verify after running: extra/strace/linux_arm64/strace must be "statically
# linked" (ARM aarch64) and `strace -V` must report the pinned version.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
OUT_DIR="${REPO_ROOT}/extra/strace"

# docker platform : output directory name under extra/strace
# amd64 is deliberately absent -- it keeps the existing committed binary.
PLATFORMS=("linux/arm64:linux_arm64")

for entry in "${PLATFORMS[@]}"; do
  platform="${entry%%:*}"
  dirname="${entry##*:}"
  echo "==> building static strace for ${platform}"
  tmp_out="${OUT_DIR}/${dirname}.tmp"
  rm -rf "${tmp_out}"
  # Export the single /strace artifact out of the scratch stage.
  docker buildx build \
    --platform "${platform}" \
    --target export \
    --output "type=local,dest=${tmp_out}" \
    "${SCRIPT_DIR}"
  mkdir -p "${OUT_DIR}/${dirname}"
  mv "${tmp_out}/strace" "${OUT_DIR}/${dirname}/strace"
  rm -rf "${tmp_out}"
  chmod +x "${OUT_DIR}/${dirname}/strace"
  echo "    -> ${OUT_DIR}/${dirname}/strace"
  file "${OUT_DIR}/${dirname}/strace" || true
done

echo "==> done. Artifacts:"
for entry in "${PLATFORMS[@]}"; do
  dirname="${entry##*:}"
  echo "    ${OUT_DIR}/${dirname}/strace"
done
