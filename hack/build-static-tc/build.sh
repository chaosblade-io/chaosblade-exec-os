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

# Build the bundled static tc for linux/amd64 and linux/arm64 and drop the
# artifacts into extra/tc/<platform>/tc, ready for `make linux_amd64` etc.
#
# Requires: docker with buildx. Artifacts are consumed by the Makefile, which
# copies extra/tc/<platform>/tc next to chaos_os.
#
# Verify after running: each extra/tc/<platform>/tc must be "statically linked"
# and `tc -V` must report 6.6.0.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
OUT_DIR="${REPO_ROOT}/extra/tc"

# docker platform : output directory name under extra/tc
PLATFORMS=("linux/amd64:linux_amd64" "linux/arm64:linux_arm64")

for entry in "${PLATFORMS[@]}"; do
  platform="${entry%%:*}"
  dirname="${entry##*:}"
  echo "==> building static tc for ${platform}"
  tmp_out="${OUT_DIR}/${dirname}.tmp"
  rm -rf "${tmp_out}"
  # Export the single /tc artifact out of the scratch stage.
  docker buildx build \
    --platform "${platform}" \
    --target export \
    --output "type=local,dest=${tmp_out}" \
    "${SCRIPT_DIR}"
  mkdir -p "${OUT_DIR}/${dirname}"
  mv "${tmp_out}/tc" "${OUT_DIR}/${dirname}/tc"
  rm -rf "${tmp_out}"
  chmod +x "${OUT_DIR}/${dirname}/tc"
  echo "    -> ${OUT_DIR}/${dirname}/tc"
  file "${OUT_DIR}/${dirname}/tc" || true
done

echo "==> done. Artifacts:"
for entry in "${PLATFORMS[@]}"; do
  dirname="${entry##*:}"
  echo "    ${OUT_DIR}/${dirname}/tc"
done
