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

# Build a statically linked strace for linux/arm64 to bundle with chaos_os.
#
# Why: extra/strace used to be a single x86-64 (glibc) binary that the Makefile
# copied into every platform's output, so arm64 artifacts shipped an x86-64
# strace which cannot execute on arm64 nodes ("exec format error"). The
# strace-based kernel delay/error actions (exec/kernel, registered only on Linux
# via exec/model/model_linux.go) resolve strace by absolute path
# util.GetProgramPath()/strace, so a wrong-arch binary silently breaks them.
#
# This recipe produces the missing arm64 binary only. The existing amd64 binary
# (extra/strace/linux_amd64/strace, glibc strace ~5.8) is left untouched because
# it is already validated in production; the two run on their own arch and never
# mix, and strace's ptrace interface is stable across versions, so a differing
# arm64 version carries no cross-arch risk.
#
# Usage: run build.sh in this directory (docker buildx, arm64).
# NOTE: this recipe must be executed once in a docker buildx environment to
# produce extra/strace/linux_arm64/strace.
ARG ALPINE_VERSION=3.19
ARG STRACE_VERSION=6.6
FROM alpine:${ALPINE_VERSION} AS builder

# Re-declare the global ARG so this stage can use it.
ARG STRACE_VERSION

# strace needs the kernel headers (linux/ioctl.h, asm/types.h, ...) plus the
# toolchain; xz to unpack the release tarball.
RUN apk add --no-cache build-base linux-headers curl tar xz

WORKDIR /src
RUN curl -fsSL "https://github.com/strace/strace/releases/download/v${STRACE_VERSION}/strace-${STRACE_VERSION}.tar.xz" \
      | tar xJ

WORKDIR /src/strace-${STRACE_VERSION}
# --enable-mpers=no: skip multi-arch personality support (unneeded here and it
# complicates static builds). Static musl link keeps the artifact independent of
# the runtime base image's libc.
RUN ./configure --enable-mpers=no --disable-gcc-Werror
RUN make -j"$(nproc)" LDFLAGS="-static"

# Smoke check inside the builder: must be static and report the pinned version.
RUN file src/strace | grep -q "statically linked" \
 && ./src/strace -V | grep -q "${STRACE_VERSION}"

# Minimal stage that exposes only the artifact, for `buildx --output`.
FROM scratch AS export
# Re-declare the global ARG so the COPY source path resolves the version.
ARG STRACE_VERSION
COPY --from=builder /src/strace-${STRACE_VERSION}/src/strace /strace
