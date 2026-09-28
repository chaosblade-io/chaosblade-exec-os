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

# Build a pinned, statically linked iproute2 `tc` for bundling with chaos_os.
#
# Why: since iproute2 v6.9.0 (commit "netem: use 64 bit value for latency and
# jitter", 2024-03-13) tc encodes netem delay into the 64-bit
# TCA_NETEM_LATENCY64 attribute and leaves the 32-bit opt.latency zero. Kernels
# older than 4.15 do not understand LATENCY64, read opt.latency (0) and silently
# ignore the unknown netlink attribute, so `blade create network delay` creates
# the qdisc (limit 1000) but applies no delay -- and reports success.
#
# Pinning tc to 6.6.0 (the version shipped by alpine 3.19, safely below the 6.9
# boundary; 6.6 -> 6.8 adds nothing ChaosBlade needs) and shipping it next to
# chaos_os removes the dependency on whatever iproute2 the runtime base image
# happens to carry. Static (musl) linking keeps it independent of the base
# image's libc as well.
#
# Usage: run build.sh in this directory (docker buildx, amd64 + arm64).
# NOTE: this recipe must be executed once in a docker buildx environment to
# produce extra/tc/<platform>/tc; iproute2's static dependency set can need
# minor package-name tweaks per alpine release.
ARG ALPINE_VERSION=3.19
ARG IPROUTE2_VERSION=6.6.0
FROM alpine:${ALPINE_VERSION} AS builder

# Re-declare the global ARG so this stage can use it.
ARG IPROUTE2_VERSION

# Build toolchain + the static (.a) variant of every tc dependency.
# elfutils/libbpf are intentionally NOT installed: without them configure
# reports "ELF support: no" and tc is built without the eBPF object loader,
# which is the only part pulling in libelf (and transitively zstd). The
# netem/qdisc/class/filter paths ChaosBlade uses need none of it.
RUN apk add --no-cache \
        build-base linux-headers flex bison curl tar \
        libmnl-dev libmnl-static \
        libcap-dev libcap-static \
        zlib-dev zlib-static

WORKDIR /src
RUN curl -fsSL "https://www.kernel.org/pub/linux/utils/net/iproute2/iproute2-${IPROUTE2_VERSION}.tar.gz" \
      | tar xz

WORKDIR /src/iproute2-${IPROUTE2_VERSION}
RUN ./configure

# Build only lib+tc and static-link them. Building the whole tree fails on misc
# tools (e.g. rtmon) whose Makefiles do not forward the static libs, so SUBDIRS
# is narrowed to the two directories tc actually needs.
RUN make SUBDIRS="lib tc" \
        CCOPTS="-static" \
        LDFLAGS="-static" \
        LDLIBS="-lmnl -lcap -lz -lm"

# Smoke check inside the builder: must be static and report the pinned version.
RUN file tc/tc | grep -q "statically linked" \
 && ./tc/tc -V | grep -q "${IPROUTE2_VERSION}"

# Minimal stage that exposes only the artifact, for `buildx --output`.
FROM scratch AS export
# Re-declare the global ARG so the COPY source path resolves the version.
ARG IPROUTE2_VERSION
COPY --from=builder /src/iproute2-${IPROUTE2_VERSION}/tc/tc /tc
