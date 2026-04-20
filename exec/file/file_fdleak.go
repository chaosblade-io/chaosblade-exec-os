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
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"golang.org/x/sys/unix"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
)

const (
	FdLeakBin           = "chaos_fdleak"
	defaultFdLeakPrefix = "chaos_fdleak_"
)

var fdleakLocalChannel = channel.NewLocalChannel()

func NewFileFdleakActionSpec() spec.ExpActionCommandSpec {
	return &FileFdleakActionCommandSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "percent",
					Desc:     "percentage of disk space to occupy with leaked file data, e.g. 50 or 50%",
					Required: true,
				},
			},
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:    "directory",
					Desc:    "directory for temporary files; defaults to OS temp directory",
					Default: "",
				},
				&spec.ExpFlag{
					Name:    "prefix",
					Desc:    "filename prefix for temp files",
					Default: defaultFdLeakPrefix,
				},
			},
			ActionExecutor: &FileFdleakExecutor{},
			ActionExample: `
# Occupy about 50% of disk space with a leaked unlinked file
blade create file fdleak --percent 50 --directory /tmp --prefix chaos_test_`,
			ActionPrograms:    []string{FdLeakBin},
			ActionCategories:  []string{category.SystemFile},
			ActionProcessHang: true,
		},
	}
}

type FileFdleakActionCommandSpec struct {
	spec.BaseExpActionCommandSpec
}

func (*FileFdleakActionCommandSpec) Name() string {
	return "fdleak"
}

func (*FileFdleakActionCommandSpec) Aliases() []string {
	return []string{}
}

func (*FileFdleakActionCommandSpec) ShortDesc() string {
	return "File descriptor leak (unlinked open files)"
}

func (f *FileFdleakActionCommandSpec) LongDesc() string {
	if f.ActionLongDesc != "" {
		return f.ActionLongDesc
	}
	return "Creates temporary files, writes random data, unlinks paths while keeping fds open to simulate fd leaks"
}

func (*FileFdleakActionCommandSpec) Categories() []string {
	return []string{category.SystemFile}
}

type FileFdleakExecutor struct {
	channel spec.Channel
}

func (*FileFdleakExecutor) Name() string {
	return "fdleak"
}

func (e *FileFdleakExecutor) SetChannel(channel spec.Channel) {
	e.channel = channel
}

func (e *FileFdleakExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	dir := model.ActionFlags["directory"]
	if dir == "" {
		dir = os.TempDir()
	}
	prefix := model.ActionFlags["prefix"]
	if prefix == "" {
		prefix = defaultFdLeakPrefix
	}

	if _, ok := spec.IsDestroy(ctx); ok {
		return e.stop(ctx, dir, prefix)
	}

	percent, perr := parsePercent(model.ActionFlags["percent"])
	if perr != nil {
		log.Errorf(ctx, "fdleak: %v", perr)
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "percent", "", perr.Error())
	}

	if err := os.MkdirAll(dir, 0o750); err != nil {
		log.Errorf(ctx, "fdleak: mkdir %s: %v", dir, err)
		return spec.ResponseFailWithFlags(spec.ParameterInvalid, "directory", dir, err.Error())
	}

	return e.startByPercent(ctx, percent, dir, prefix)
}

func (e *FileFdleakExecutor) startByPercent(ctx context.Context, percent int, dir, prefix string) *spec.Response {
	totalBytes, availableBytes, err := getDiskSpaceInfo(dir)
	if err != nil {
		log.Errorf(ctx, "fdleak: get disk info for %s: %v", dir, err)
		return spec.ReturnFail(spec.OsCmdExecFailed, fmt.Sprintf("get disk info: %v", err))
	}

	targetBytes := computeTargetBytes(availableBytes, percent)
	if targetBytes <= 0 {
		log.Warnf(ctx, "fdleak: nothing to leak (targetBytes=%d, availableBytes=%d)", targetBytes, availableBytes)
		return spec.ReturnSuccess(ctx.Value(spec.Uid))
	}

	log.Infof(ctx, "fdleak: will occupy %d bytes (%d%% of %d available, %d total) on %s", targetBytes, percent, availableBytes, totalBytes, dir)

	pattern := prefix
	if !strings.HasSuffix(pattern, "*") {
		pattern += "*"
	}

	f, err := leakOneUnlinkedFD(dir, pattern, targetBytes)
	if err != nil {
		log.Errorf(ctx, "fdleak: %v", err)
		return spec.ReturnFail(spec.OsCmdExecFailed, err.Error())
	}

	log.Infof(ctx, "fdleak: holding 1 unlinked open file (%d bytes); blocking until destroy", targetBytes)
	return e.blockUntilSignal(ctx, []*os.File{f})
}

// blockUntilSignal blocks until SIGTERM/SIGINT, then closes all files gracefully.
// Note: exec.Destroy sends SIGKILL (kill -9), which terminates the process immediately
// without running this cleanup. In that case, the OS reclaims all open FDs automatically
// when the process exits. This signal handler is a best-effort for graceful shutdowns.
func (e *FileFdleakExecutor) blockUntilSignal(ctx context.Context, openFiles []*os.File) *spec.Response {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	sig := <-sigCh
	log.Infof(ctx, "fdleak: received signal %v, closing %d leaked fds", sig, len(openFiles))
	e.closeAll(openFiles)
	return spec.ReturnSuccess(ctx.Value(spec.Uid))
}

// leakOneUnlinkedFD creates a temp file under dir, occupies size bytes of disk space,
// unlinks the path, and returns the open *os.File.
// It first tries fallocate for fast allocation; if unsupported, falls back to writing zeros.
// The file path is unlinked immediately after creation (before writing data), so that
// even if the process is killed (SIGKILL) during the write, the OS will automatically
// reclaim disk space when the fd is closed — no residual files left on disk.
func leakOneUnlinkedFD(dir, pattern string, size int64) (*os.File, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}

	// Ensure file is closed if any subsequent operation fails
	closeOnError := true
	defer func() {
		if closeOnError && f != nil {
			_ = f.Close()
		}
	}()

	// Unlink the file path first, before writing any data.
	// This ensures that if the process is killed during write, the OS reclaims
	// disk space automatically when the fd is closed — no orphan files remain.
	name := f.Name()
	if err := os.Remove(name); err != nil {
		return nil, fmt.Errorf("unlink %s: %w", name, err)
	}

	// Try fallocate first (instant disk space allocation without writing data)
	if fallocErr := tryFallocate(int(f.Fd()), size); fallocErr != nil {
		// Fallocate not supported (e.g., tmpfs, some filesystems), fall back to writing zeros
		if _, writeErr := io.CopyN(f, zeroReader{}, size); writeErr != nil {
			return nil, fmt.Errorf("write data to temp file: %w", writeErr)
		}
	}

	// All operations succeeded, keep the file descriptor open
	closeOnError = false
	return f, nil
}

// zeroReader is an io.Reader that returns zeros (much faster than crypto/rand).
type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func (e *FileFdleakExecutor) closeAll(files []*os.File) {
	for _, f := range files {
		if f != nil {
			_ = f.Close()
		}
	}
}

func (e *FileFdleakExecutor) stop(ctx context.Context, dir, prefix string) *spec.Response {
	ch := e.channel
	if ch == nil {
		ch = fdleakLocalChannel
	}
	ctx = context.WithValue(ctx, "bin", FdLeakBin)
	resp := exec.Destroy(ctx, ch, "file fdleak")

	// Best-effort cleanup: after killing the process, try to remove any leftover
	// temp files. On normal filesystems this works reliably. On overlay FS, files
	// may be hidden by whiteouts and not found by Glob — this is a known limitation.
	e.cleanupTempFiles(ctx, dir, prefix)

	return resp
}

// cleanupTempFiles removes temp files matching the prefix pattern in the given directory.
func (e *FileFdleakExecutor) cleanupTempFiles(ctx context.Context, dir, prefix string) {
	pattern := filepath.Join(dir, prefix+"*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		log.Warnf(ctx, "fdleak cleanup: glob %s: %v", pattern, err)
		return
	}
	for _, path := range matches {
		if err := os.Remove(path); err != nil {
			log.Warnf(ctx, "fdleak cleanup: remove %s: %v", path, err)
		} else {
			log.Infof(ctx, "fdleak cleanup: removed %s", path)
		}
	}
}

func parsePercent(percentStr string) (int, error) {
	percentStr = strings.TrimSpace(percentStr)
	if percentStr == "" {
		return 0, fmt.Errorf("percent is required")
	}
	p := strings.TrimSuffix(percentStr, "%")
	p = strings.TrimSpace(p)
	n, e := strconv.Atoi(p)
	if e != nil || n <= 0 || n > 100 {
		return 0, fmt.Errorf("percent must be an integer between 1 and 100")
	}
	return n, nil
}

// computeTargetBytes calculates the target bytes to occupy based on available space and percent.
// It performs arithmetic in uint64 to avoid overflow on very large filesystems, then clamps
// the result to math.MaxInt64 before converting to int64.
func computeTargetBytes(availableBytes uint64, percent int) int64 {
	// Use division-first to avoid uint64 overflow: availableBytes / 100 * percent
	// loses at most (percent-1) bytes of precision, which is negligible.
	target := availableBytes / 100 * uint64(percent)
	if target > availableBytes {
		target = availableBytes
	}
	if target > uint64(math.MaxInt64) {
		target = uint64(math.MaxInt64)
	}
	return int64(target)
}

// getDiskSpaceInfo returns (totalBytes, availableBytes, error) for the filesystem containing path.
func getDiskSpaceInfo(path string) (uint64, uint64, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, 0, err
	}
	totalBytes := uint64(stat.Blocks) * uint64(stat.Bsize)
	availableBytes := uint64(stat.Bavail) * uint64(stat.Bsize)
	return totalBytes, availableBytes, nil
}
