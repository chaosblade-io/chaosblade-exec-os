/*
 * Copyright 2025 The ChaosBlade Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Coverage:
//  1. A container CPU entrypoint without usable v1 controllers fails before
//     taskset or the burn goroutines start.
//  2. A non-container entrypoint does not require cgroups and a v2 entrypoint
//     is not subjected to v1 controller rules.
//
// Approach: an empty temp dir is not recognized as a cgroup mount by statfs,
// while the current PID supplies a real proc entry. No controller is modified.
// An invalid cpu-list proves preflight runs before taskset.
package cpu

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
)

func TestCPUStartRejectsMissingV1Controllers(t *testing.T) {
	ctx := context.WithValue(context.Background(), channel.NSTargetFlagName, strconv.Itoa(os.Getpid()))
	ctx = context.WithValue(ctx, "cgroup-root", t.TempDir())
	response := (&cpuExecutor{}).start(ctx, "invalid-cpu-list", 1, 20, 0, "")
	if response.Success || !strings.Contains(response.Err, "cpu preflight failed") {
		t.Fatalf("CPU start did not fail before taskset/burn: %+v", response)
	}
}

func TestCPUPreflightHostAndV2(t *testing.T) {
	if err := validateContainerCPU(context.Background()); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "cgroup.controllers"), []byte("cpu memory"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), channel.NSTargetFlagName, strconv.Itoa(os.Getpid()))
	ctx = context.WithValue(ctx, "cgroup-root", root)
	if err := validateContainerCPU(ctx); err != nil {
		t.Fatalf("v1 preflight was applied to v2: %v", err)
	}
}
