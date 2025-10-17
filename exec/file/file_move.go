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
	"fmt"
	"path"

	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
)

const (
	MoveFileBin = "chaos_movefile"
	suffix      = ".chaos-blade-backup"
)

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
# Move the file /home/logs/nginx.log to /tmp
blade create file move --filepath /home/logs/nginx.log --target /tmp

# Force Move the file /home/logs/nginx.log to /tmp
blade create file move --filepath /home/logs/nginx.log --target /tmp --force

# Move the file /home/logs/nginx.log to /tmp and automatically create directories that don't exist
blade create file move --filepath /home/logs/nginx.log --target /tmp --auto-create-dir
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
	return "move"
}

func (f *FileMoveActionExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	commands := []string{"mv", "mkdir"}
	if response, ok := f.channel.IsAllCommandsAvailable(ctx, commands); !ok {
		return response
	}

	filepath := model.ActionFlags["filepath"]

	target := model.ActionFlags["target"]

	if _, ok := spec.IsDestroy(ctx); ok {
		return f.stop(filepath, target, ctx)
	}

	force := model.ActionFlags["force"] == "true"
	autoCreateDir := model.ActionFlags["auto-create-dir"] == "true"

	if !force {
		targetFile := path.Join(target, "/", path.Base(filepath))
		if exec.CheckFilepathExists(ctx, f.channel, targetFile) {
			log.Errorf(ctx, "`%s`: target file already exists", targetFile)
			return spec.ResponseFailWithFlags(spec.ParameterInvalid, "target", targetFile, "the target file already exists")
		}
	}
	return f.start(filepath, target, force, autoCreateDir, ctx)
}

func (f *FileMoveActionExecutor) start(filepath, target string, force, autoCreateDir bool, ctx context.Context) *spec.Response {
	var response *spec.Response

	if autoCreateDir && !exec.CheckFilepathExists(ctx, f.channel, target) {
		response = f.channel.Run(ctx, "mkdir", fmt.Sprintf(`-p %s`, target))
		if !response.Success {
			return response
		}
	}

	if force {
		// backup
		_ = f.channel.Run(ctx, "cp", fmt.Sprintf(`"%s" "%s"`, path.Join(target, path.Base(filepath)),
			path.Join(target, path.Base(filepath)+suffix)))

		response = f.channel.Run(ctx, "mv", fmt.Sprintf(`-f "%s" "%s"`, filepath, target))
	} else {
		response = f.channel.Run(ctx, "mv", fmt.Sprintf(`"%s" "%s"`, filepath, target))
	}
	return response
}

func (f *FileMoveActionExecutor) stop(filepath, target string, ctx context.Context) *spec.Response {
	origin := path.Join(target, "/", path.Base(filepath))
	response := f.channel.Run(ctx, "mv", fmt.Sprintf(`-f "%s" "%s"`, origin, path.Dir(filepath)))
	if response.Success {
		// restore backup
		_ = f.channel.Run(ctx, "mv", fmt.Sprintf(`"%s" "%s"`, path.Join(target, path.Base(filepath)+suffix),
			path.Join(target, path.Base(filepath))))
	}
	return response
}

func (f *FileMoveActionExecutor) SetChannel(channel spec.Channel) {
	f.channel = channel
}
