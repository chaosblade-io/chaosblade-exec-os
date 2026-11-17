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

package disk

import (
	"context"
	"strconv"
	"syscall"
	"testing"
)

func statfs(blocks, bfree uint64) *syscall.Statfs_t {
	return &syscall.Statfs_t{Blocks: blocks, Bfree: bfree, Bsize: 4096}
}

func TestFillDeficitBytes(t *testing.T) {
	const bs = 4096
	cases := []struct {
		name    string
		stat    *syscall.Statfs_t
		percent int
		want    int64
		wantErr bool
	}{
		{"percent100 fills all free incl reserved", statfs(1000, 400), 100, 400 * bs, false},
		{"percent80 partial", statfs(1000, 400), 80, 200 * bs, false},
		{"already at target errors", statfs(1000, 400), 60, 0, true},
		{"above target errors", statfs(1000, 400), 50, 0, true},
		{"reserved blocks are fillable not used", statfs(1000, 100), 100, 100 * bs, false},
		{"deficit clamped to free", statfs(1000, 400), 150, 400 * bs, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := fillDeficitBytes(c.stat, c.percent)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got deficit=%d", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != c.want {
				t.Errorf("deficit=%d want=%d", got, c.want)
			}
		})
	}
}

func TestCalculateFileSizePercentNoRoundingHeadroom(t *testing.T) {
	old := getSysStatFunc
	defer func() { getSysStatFunc = old }()
	// free = 30000*4096 = 122880000 B = 117.1875 MB; percent=100 must fill floor(free)=117 MB,
	// leaving <1MB, not the ~0.5% headroom the old %.2f rounding produced.
	getSysStatFunc = func(string) *syscall.Statfs_t { return statfs(100000, 30000) }
	got, err := calculateFileSize(context.Background(), "/app", "", "100", "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "117" {
		t.Errorf("size=%s want=117", got)
	}
}

func TestCalculateFileSizePercentAlreadyFull(t *testing.T) {
	old := getSysStatFunc
	defer func() { getSysStatFunc = old }()
	getSysStatFunc = func(string) *syscall.Statfs_t { return statfs(1000, 100) } // 90% used
	if _, err := calculateFileSize(context.Background(), "/app", "", "80", ""); err == nil {
		t.Fatal("expected error when usage already >= percent")
	}
}

func TestCalculateFileSizeReserveUsesTrueFree(t *testing.T) {
	old := getSysStatFunc
	defer func() { getSysStatFunc = old }()
	// free = 400*4096 = 1638400 B = 1.5625 MB; reserve=1 MB -> fill floor(1.5625-1)=0 MB.
	getSysStatFunc = func(string) *syscall.Statfs_t { return statfs(1000, 400) }
	got, err := calculateFileSize(context.Background(), "/app", "", "", "1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	want := strconv.Itoa(int((400*4096)/(1024*1024) - 1))
	if got != want {
		t.Errorf("size=%s want=%s", got, want)
	}
}
