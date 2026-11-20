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
//  1. Container ram/cache modes fail before memory allocation or tmpfs mount
//     when no usable v1 controller exists.
//  2. A statistics load failure must not fall back to host memory for a
//     container experiment.
//
// Approach: only the channel command availability check is allowed; the other
// embedded channel interface methods are nil, so any premature mount/dd call
// fails the test. The temp root is not a real cgroup and creates no pressure.
package mem

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

type preflightChannel struct{ spec.Channel }

func (preflightChannel) IsAllCommandsAvailable(context.Context, []string) (*spec.Response, bool) {
	return nil, true
}

func TestMemoryExecRejectsMissingV1Controllers(t *testing.T) {
	for _, mode := range []string{"ram", "cache"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), channel.NSTargetFlagName, strconv.Itoa(os.Getpid()))
			model := &spec.ExpModel{ActionFlags: map[string]string{
				"mode": mode, "mem-percent": "20", "cgroup-root": t.TempDir(),
			}}
			executor := &memExecutor{channel: preflightChannel{}}
			response := executor.Exec("preflight", ctx, model)
			if response.Success || !strings.Contains(response.Err, "memory preflight failed") {
				t.Fatalf("memory start did not fail before allocation/mount: %+v", response)
			}
		})
	}
}
