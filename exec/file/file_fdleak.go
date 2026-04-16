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
	"os"
	"os/signal"
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
	if _, ok := spec.IsDestroy(ctx); ok {
		return e.stop(ctx)
	}

	dir := model.ActionFlags["directory"]
	if dir == "" {
		dir = os.TempDir()
	}
	prefix := model.ActionFlags["prefix"]
	if prefix == "" {
		prefix = defaultFdLeakPrefix
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

func (e *FileFdleakExecutor) Check(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	if _, ok := spec.IsDestroy(ctx); ok {
		return e.stop(ctx)
	}

	if _, perr := parsePercent(model.ActionFlags["percent"]); perr != nil {
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "percent", "", perr.Error())
	}

	dir := model.ActionFlags["directory"]
	if dir == "" {
		dir = os.TempDir()
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return spec.ResponseFailWithFlags(spec.ParameterInvalid, "directory", dir, err.Error())
	}

	if model.ActionFlags["percent"] != "" {
		if _, _, err := getDiskSpaceInfo(dir); err != nil {
			return spec.ResponseFailWithResult(spec.ActionNotSupport, fmt.Sprintf("get disk info: %v", err))
		}
	}

	return spec.ReturnSuccess(ctx.Value(spec.Uid))
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

	// Write zeros to occupy disk space (much faster than crypto/rand)
	if _, writeErr := io.CopyN(f, zeroReader{}, size); writeErr != nil {
		return nil, fmt.Errorf("write data to temp file: %w", writeErr)
	}

	name := f.Name()
	if err := os.Remove(name); err != nil {
		return nil, fmt.Errorf("unlink %s: %w", name, err)
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

func (e *FileFdleakExecutor) stop(ctx context.Context) *spec.Response {
	ch := e.channel
	if ch == nil {
		ch = fdleakLocalChannel
	}
	ctx = context.WithValue(ctx, "bin", FdLeakBin)
	return exec.Destroy(ctx, ch, "file fdleak")
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
func computeTargetBytes(availableBytes uint64, percent int) int64 {
	target := int64(availableBytes * uint64(percent) / 100)
	if uint64(target) > availableBytes {
		target = int64(availableBytes)
	}
	return target
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
