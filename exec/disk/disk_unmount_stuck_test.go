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
	"os"
	"path"
	"syscall"
	"testing"
	"time"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

func TestUnmountStuckActionSpec(t *testing.T) {
	s := NewUnmountStuckActionSpec()
	if s.Name() != "unmount_stuck" {
		t.Fatalf("expected name unmount_stuck, got %s", s.Name())
	}
	if s.ShortDesc() == "" {
		t.Fatal("ShortDesc should not be empty")
	}
	if s.LongDesc() == "" {
		t.Fatal("LongDesc should not be empty")
	}
	if s.Executor() == nil {
		t.Fatal("Executor should not be nil")
	}
	if len(s.Programs()) == 0 {
		t.Fatal("Programs should not be empty")
	}
	if s.Programs()[0] != UnmountStuckBin {
		t.Fatalf("expected program %s, got %s", UnmountStuckBin, s.Programs()[0])
	}
}

func TestUnmountStuckExec_MissingPath(t *testing.T) {
	executor := &UnmountStuckExecutor{}
	executor.SetChannel(channel.NewLocalChannel())

	ctx := context.Background()
	model := &spec.ExpModel{
		ActionFlags: map[string]string{},
	}
	resp := executor.Exec("test-uid", ctx, model)
	if resp.Success {
		t.Fatal("expected failure when path is missing")
	}
}

func TestUnmountStuckExec_InvalidPath(t *testing.T) {
	executor := &UnmountStuckExecutor{}
	executor.SetChannel(channel.NewLocalChannel())

	ctx := context.Background()
	model := &spec.ExpModel{
		ActionFlags: map[string]string{
			"path": "/nonexistent_path_for_test_12345",
		},
	}
	resp := executor.Exec("test-uid", ctx, model)
	if resp.Success {
		t.Fatal("expected failure when path is not a directory")
	}
}

func TestUnmountStuckStart_CreatesSentinelFile(t *testing.T) {
	dir := t.TempDir()
	executor := &UnmountStuckExecutor{}
	executor.SetChannel(channel.NewLocalChannel())

	ctx := context.Background()

	// Run start in a goroutine since it blocks on signal
	done := make(chan *spec.Response, 1)
	go func() {
		done <- executor.start(ctx, dir)
	}()

	// Wait for sentinel file to be created
	sentinelPath := path.Join(dir, unmountStuckFile)
	deadline := time.After(3 * time.Second)
	for {
		if _, err := os.Stat(sentinelPath); err == nil {
			break
		}
		select {
		case <-deadline:
			t.Fatal("sentinel file was not created within timeout")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	// Send SIGTERM to unblock the start method
	syscall.Kill(syscall.Getpid(), syscall.SIGTERM)

	select {
	case resp := <-done:
		if !resp.Success {
			t.Fatalf("expected success response, got: %s", resp.Err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("start did not return after SIGTERM")
	}

	// After graceful shutdown, sentinel file should be cleaned up
	if _, err := os.Stat(sentinelPath); !os.IsNotExist(err) {
		t.Fatal("sentinel file should be removed after graceful shutdown")
	}
}

func TestUnmountStuckStop_CleansSentinelFile(t *testing.T) {
	dir := t.TempDir()
	executor := &UnmountStuckExecutor{}
	executor.SetChannel(channel.NewLocalChannel())

	// Create sentinel file manually
	sentinelPath := path.Join(dir, unmountStuckFile)
	f, err := os.Create(sentinelPath)
	if err != nil {
		t.Fatalf("failed to create sentinel file: %v", err)
	}
	f.Close()

	// Verify it exists
	if _, err := os.Stat(sentinelPath); err != nil {
		t.Fatalf("sentinel file should exist: %v", err)
	}

	ctx := context.Background()
	ctx = spec.SetDestroyFlag(ctx, "test-uid")
	executor.stop(ctx, dir)

	// Verify sentinel file is removed
	if _, err := os.Stat(sentinelPath); !os.IsNotExist(err) {
		t.Fatal("sentinel file should be removed after stop")
	}
}

func TestUnmountStuckStop_NoFileNoPanic(t *testing.T) {
	dir := t.TempDir()
	executor := &UnmountStuckExecutor{}
	executor.SetChannel(channel.NewLocalChannel())

	ctx := context.Background()
	ctx = spec.SetDestroyFlag(ctx, "test-uid")

	// Should not panic when sentinel file doesn't exist
	resp := executor.stop(ctx, dir)
	if !resp.Success {
		// exec.Destroy may return success with "no processes found"
		t.Logf("stop response: %v", resp)
	}
}
