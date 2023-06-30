/*
 * Copyright 1999-2020 Alibaba Group Holding Ltd.
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
	"os"
	"path"
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
)

const MoveFileBin = "chaos_movefile"

type FileMoveActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewFileMoveActionSpec() spec.ExpActionCommandSpec {
	return &FileMoveActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: fileCommFlags,
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "target",
					Desc:     "target folder",
					Required: true,
				},
				&spec.ExpFlag{
					Name:   "force",
					Desc:   "use --force flag overwrite target file",
					NoArgs: true,
				},
				&spec.ExpFlag{
					Name:   "auto-create-dir",
					Desc:   "automatically creates a directory that does not exist",
					NoArgs: true,
				},
			},
			ActionExecutor: &FileMoveActionExecutor{},
			ActionExample: `
# Move the file C:\nginx.log to D:\
blade create file move --filepath C:\nginx.log --target D:\

# Force Move the file C:\nginx.log to D:\
blade create file move --filepath C:\nginx.log --target D:\ --force

# Move the file /home/logs/nginx.log to D:\ and automatically create directories that don't exist
blade create file move --filepath C:\nginx.log --target D:\ --auto-create-dir
`,
			ActionPrograms:   []string{MoveFileBin},
			ActionCategories: []string{category.SystemFile},
		},
	}
}

func (*FileMoveActionSpec) Name() string {
	return "move"
}

func (*FileMoveActionSpec) Aliases() []string {
	return []string{}
}

func (*FileMoveActionSpec) ShortDesc() string {
	return "File move"
}

func (f *FileMoveActionSpec) LongDesc() string {
	return "File move"
}

type FileMoveActionExecutor struct {
	channel spec.Channel
}

func (*FileMoveActionExecutor) Name() string {
	return "remove"
}

func (f *FileMoveActionExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {

	filepath := model.ActionFlags["filepath"]

	target := model.ActionFlags["target"]

	if _, ok := spec.IsDestroy(ctx); ok {
		return f.stop(filepath, target, ctx)
	}

	if !exec.CheckFilepathExists(ctx, f.channel, filepath) {
		log.Errorf(ctx, "`%s`: filepath does not exist", filepath)
		return spec.ResponseFailWithFlags(spec.ParameterInvalid, "filepath", filepath, "the file does not exist")
	}

	force := model.ActionFlags["force"] == "true"
	autoCreateDir := model.ActionFlags["auto-create-dir"] == "true"
	// todo check一下如果target文件已存在，会不会强制覆盖
	if !force {
		targetFile := path.Join(target, path.Base(filepath))
		if exec.CheckFilepathExists(ctx, f.channel, targetFile) {
			log.Errorf(ctx, "`%s`: target file does not exist", targetFile)
			return spec.ResponseFailWithFlags(spec.ParameterInvalid, "target", targetFile, "the target file does not exist")
		}
	}
	return f.start(filepath, target, force, autoCreateDir, ctx)
}

func (f *FileMoveActionExecutor) start(filepath, target string, force, autoCreateDir bool, ctx context.Context) *spec.Response {
	if autoCreateDir && !exec.CheckFilepathExists(ctx, f.channel, target) {
		err := os.Mkdir(target, 0)
		if err != nil {
			return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "mkdir", err.Error())
		}
	}

	lfilePath := strings.Replace(filepath, "\\", "/", -1)
	err := os.Rename(filepath, path.Join(target, path.Base(lfilePath)))
	if err != nil {
		return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "move file", err.Error())
	}
	return spec.Success()
}

func (f *FileMoveActionExecutor) stop(filepath, target string, ctx context.Context) *spec.Response {
	lfilePath := strings.Replace(filepath, "\\", "/", -1)
	origin := path.Join(target, path.Base(lfilePath))
	err := os.Rename(origin, filepath)
	if err != nil {
		return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "move file", err.Error())
	}
	return spec.Success()
}

func (f *FileMoveActionExecutor) SetChannel(channel spec.Channel) {
	f.channel = channel
}
