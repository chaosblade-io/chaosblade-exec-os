//go:build !unix

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

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
)

var fdleakLocalChannelStub = channel.NewLocalChannel()

const FdLeakBin = "chaos_fdleak"

func NewFileFdleakActionSpec() spec.ExpActionCommandSpec {
	return &FileFdleakActionCommandSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{
				&spec.ExpFlag{Name: "percent", Desc: "percentage of disk space to occupy with leaked file data, e.g. 50 or 50%"},
			},
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{Name: "directory", Desc: "directory for temporary files", Default: ""},
				&spec.ExpFlag{Name: "prefix", Desc: "filename prefix for temp files", Default: "chaos_fdleak_"},
			},
			ActionExecutor:   &FileFdleakExecutor{},
			ActionPrograms:   []string{FdLeakBin},
			ActionCategories: []string{category.SystemFile},
		},
	}
}

type FileFdleakActionCommandSpec struct {
	spec.BaseExpActionCommandSpec
}

func (*FileFdleakActionCommandSpec) Name() string { return "fdleak" }

func (*FileFdleakActionCommandSpec) Aliases() []string { return []string{} }

func (*FileFdleakActionCommandSpec) ShortDesc() string {
	return "File descriptor leak (unlinked open files)"
}

func (f *FileFdleakActionCommandSpec) LongDesc() string {
	if f.ActionLongDesc != "" {
		return f.ActionLongDesc
	}
	return "Creates temporary files, writes random data, unlinks paths while keeping fds open"
}

func (*FileFdleakActionCommandSpec) Categories() []string {
	return []string{category.SystemFile}
}

type FileFdleakExecutor struct {
	channel spec.Channel
}

func (*FileFdleakExecutor) Name() string { return "fdleak" }

func (e *FileFdleakExecutor) SetChannel(channel spec.Channel) { e.channel = channel }

func (e *FileFdleakExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	if _, ok := spec.IsDestroy(ctx); ok {
		ch := e.channel
		if ch == nil {
			ch = fdleakLocalChannelStub
		}
		ctx = context.WithValue(ctx, "bin", FdLeakBin)
		return exec.Destroy(ctx, ch, "file fdleak")
	}
	return spec.ResponseFailWithResult(spec.ActionNotSupport, "file fdleak is only supported on linux and darwin")
}

func (e *FileFdleakExecutor) Check(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	if _, ok := spec.IsDestroy(ctx); ok {
		ch := e.channel
		if ch == nil {
			ch = fdleakLocalChannelStub
		}
		ctx = context.WithValue(ctx, "bin", FdLeakBin)
		return exec.Destroy(ctx, ch, "file fdleak")
	}
	return spec.ResponseFailWithResult(spec.ActionNotSupport, "file fdleak is only supported on linux and darwin")
}
