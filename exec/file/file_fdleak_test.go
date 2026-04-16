//go:build unix

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

package file

import (
	"os"
	"testing"
)

func TestParsePercent(t *testing.T) {
	tests := []struct {
		name      string
		percent   string
		wantValue int
		wantErr   bool
	}{
		{"valid", "50", 50, false},
		{"with suffix", "75%", 75, false},
		{"100", "100", 100, false},
		{"1", "1", 1, false},
		{"empty", "", 0, true},
		{"zero", "0", 0, true},
		{"over 100", "101", 0, true},
		{"negative", "-1", 0, true},
		{"non-number", "abc", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := parsePercent(tt.percent)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if v != tt.wantValue {
				t.Fatalf("got %d, want %d", v, tt.wantValue)
			}
		})
	}
}

func TestComputeTargetBytes(t *testing.T) {
	tests := []struct {
		name        string
		totalBytes  uint64
		percent     int
		expectBytes int64
	}{
		{"50 percent of 10GB", 10_000_000_000, 50, 5_000_000_000},
		{"100 percent of 1GB", 1_000_000_000, 100, 1_000_000_000},
		{"1 percent of 1GB", 1_000_000_000, 1, 10_000_000},
		{"0 total", 0, 50, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := int64(tt.totalBytes * uint64(tt.percent) / 100)
			if got != tt.expectBytes {
				t.Fatalf("got %d, want %d", got, tt.expectBytes)
			}
		})
	}
}

func TestLeakOneUnlinkedFD(t *testing.T) {
	const wantSize int64 = 4096

	dir := t.TempDir()
	f, err := leakOneUnlinkedFD(dir, "ut_*", wantSize)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if st.Size() != wantSize {
		t.Fatalf("file size %d, want %d", st.Size(), wantSize)
	}

	// Verify the file has been unlinked (path no longer exists)
	if _, err := os.Stat(f.Name()); !os.IsNotExist(err) {
		t.Fatalf("expected file %s to be unlinked, but it still exists", f.Name())
	}
}

func TestGetDiskTotalBytes(t *testing.T) {
	dir := t.TempDir()
	total, err := getDiskTotalBytes(dir)
	if err != nil {
		t.Fatalf("getDiskTotalBytes(%s) error: %v", dir, err)
	}
	if total == 0 {
		t.Fatal("getDiskTotalBytes returned 0")
	}
	t.Logf("disk total bytes for %s: %d (%.2f GB)", dir, total, float64(total)/(1<<30))
}
