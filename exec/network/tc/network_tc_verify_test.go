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

package tc

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

// mockChannel only implements Run; the embedded nil spec.Channel covers the
// rest of the interface (unused by these tests).
type mockChannel struct {
	spec.Channel
	response *spec.Response
	lastArgs string
}

func (m *mockChannel) Run(ctx context.Context, script, args string) *spec.Response {
	m.lastArgs = args
	return m.response
}

func TestNetemKeywords(t *testing.T) {
	tests := []struct {
		name      string
		classRule string
		want      []string
	}{
		{"delay", "netem delay 3000ms 0ms", []string{"delay"}},
		{"delay with distribution", "netem delay 100ms 10ms distribution normal", []string{"delay"}},
		{"loss", "netem loss 10%", []string{"loss"}},
		{"duplicate", "netem duplicate 1%", []string{"duplicate"}},
		{"corrupt", "netem corrupt 5%", []string{"corrupt"}},
		{"reorder", "netem reorder 10% 50%", []string{"reorder"}},
		{"reorder with delay", "netem reorder 10% 50% delay 100ms", []string{"reorder", "delay"}},
		{"zero delay is skipped", "netem delay 0ms 0ms", []string{}},
		{"zero loss is skipped", "netem loss 0%", []string{}},
		{"reorder with zero delay", "netem reorder 10% 50% delay 0ms", []string{"reorder"}},
		{"no netem attribute", "handle 1: prio bands 4", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := netemKeywords(tt.classRule); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("netemKeywords(%q) = %v, want %v", tt.classRule, got, tt.want)
			}
		})
	}
}

func TestVerifyNetemApplied(t *testing.T) {
	ctx := context.Background()

	t.Run("delay applied passes", func(t *testing.T) {
		cl := &mockChannel{response: spec.ReturnSuccess(
			"qdisc netem 800f: root refcnt 2 limit 1000 delay 3000ms 0ms")}
		resp := verifyNetemApplied(ctx, cl, "eth0", "netem delay 3000ms 0ms")
		if !resp.Success {
			t.Fatalf("expected success when delay is present, got: %s", resp.Err)
		}
		if !strings.Contains(cl.lastArgs, "qdisc show dev eth0") {
			t.Errorf("expected a qdisc read-back, got args: %s", cl.lastArgs)
		}
	})

	// The core regression guard: tc exits 0 but the kernel dropped the 64-bit
	// latency attribute, leaving only `limit 1000`. This must NOT pass silently.
	t.Run("delay silently dropped fails", func(t *testing.T) {
		cl := &mockChannel{response: spec.ReturnSuccess(
			"qdisc netem 800f: root refcnt 2 limit 1000")}
		resp := verifyNetemApplied(ctx, cl, "eth0", "netem delay 3000ms 0ms")
		if resp.Success {
			t.Fatal("expected failure when delay attribute is missing from read-back")
		}
		if resp.Code != spec.UnexpectedStatus.Code {
			t.Errorf("expected code %d, got %d", spec.UnexpectedStatus.Code, resp.Code)
		}
		if !strings.Contains(resp.Err, "did not take effect") {
			t.Errorf("expected an explanatory error, got: %s", resp.Err)
		}
	})

	t.Run("read-back failure does not mask injection", func(t *testing.T) {
		cl := &mockChannel{response: spec.ReturnFail(spec.OsCmdExecFailed, "boom")}
		resp := verifyNetemApplied(ctx, cl, "eth0", "netem delay 3000ms 0ms")
		if !resp.Success {
			t.Fatalf("a failed read-back must not fail the injection, got: %s", resp.Err)
		}
	})

	t.Run("no netem attribute skips verification", func(t *testing.T) {
		cl := &mockChannel{response: spec.ReturnSuccess("should not be called")}
		resp := verifyNetemApplied(ctx, cl, "eth0", "handle 1: prio bands 4")
		if !resp.Success {
			t.Fatalf("expected success with nothing to verify, got: %s", resp.Err)
		}
		if cl.lastArgs != "" {
			t.Errorf("expected no qdisc read-back, got args: %s", cl.lastArgs)
		}
	})
}

// In the unit-test environment no tc is bundled next to the test binary, so the
// helpers must fall back to the plain system tc.
func TestTcFallbackWithoutBundle(t *testing.T) {
	if hasBundledTc() {
		t.Skip("a bundled tc exists next to the test binary; fallback path not exercised")
	}
	if got := tcCommand(); got != tcName {
		t.Errorf("tcCommand() = %q, want %q", got, tcName)
	}
	if got := tcAvailabilityCommands(); !reflect.DeepEqual(got, []string{tcName, "head"}) {
		t.Errorf("tcAvailabilityCommands() = %v, want [tc head]", got)
	}
}
