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

package exec

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
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
# Move the file /home/logs/nginx.log to /tmp
blade create file move --filepath /home/logs/nginx.log --target /tmp

# Force Move the file /home/logs/nginx.log to /temp
blade create file move --filepath /home/logs/nginx.log --target /tmp --force

# Move the file /home/logs/nginx.log to /temp/ and automatically create directories that don't exist
blade create file move --filepath /home/logs/nginx.log --target /temp --auto-create-dir
`,
			ActionPrograms: []string{MoveFileBin},
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
	return "chmod"
}

func (f *FileMoveActionExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	err := checkMoveFileExpEnv()
	if err != nil {
		return spec.ReturnFail(spec.Code[spec.CommandNotFound], err.Error())
	}

	if f.channel == nil {
		return spec.ReturnFail(spec.Code[spec.ServerError], "channel is nil")
	}

	filepath := model.ActionFlags["filepath"]
	target := model.ActionFlags["target"]

	if _, ok := spec.IsDestroy(ctx); ok {
		return f.stop(filepath, target, ctx)
	}

	if !util.IsExist(filepath) {
		return spec.ReturnFail(spec.Code[spec.IllegalParameters],
			fmt.Sprintf("the %s file does not exist", filepath))
	}

	force := model.ActionFlags["force"] == "true"
	autoCreateDir := model.ActionFlags["auto-create-dir"] == "true"

	if !force {
		targetFile := path.Join(target, "/", path.Base(filepath))
		if util.IsExist(targetFile) {
			return spec.ReturnFail(spec.Code[spec.IllegalParameters],
				fmt.Sprintf("the [%s] target file is exist", targetFile))
		}
	}
	return f.start(filepath, target, force, autoCreateDir, ctx)
}

func (f *FileMoveActionExecutor) start(filepath, target string, force, autoCreateDir bool, ctx context.Context) *spec.Response {
	flags := fmt.Sprintf(`--start --filepath "%s" --target "%s" --debug=%t`, filepath, target, util.Debug)
	if force {
		flags = fmt.Sprintf(`%s --force=true`, flags)
	}
	if autoCreateDir {
		flags = fmt.Sprintf(`%s --auto-create-dir=true`, flags)
	}
	return f.channel.Run(ctx, path.Join(f.channel.GetScriptPath(), MoveFileBin), flags)
}

func (f *FileMoveActionExecutor) stop(filepath, target string, ctx context.Context) *spec.Response {
	filepath = strings.TrimPrefix(filepath, "'")
	filepath = strings.TrimSuffix(filepath, "'")
	target = strings.TrimPrefix(target, "'")
	target = strings.TrimSuffix(target, "'")
	flags := fmt.Sprintf(`--stop --filepath "%s" --target "%s" --debug=%t`, filepath, target, util.Debug)
	return f.channel.Run(ctx, path.Join(f.channel.GetScriptPath(), MoveFileBin), flags)
}

func (f *FileMoveActionExecutor) SetChannel(channel spec.Channel) {
	f.channel = channel
}

func checkMoveFileExpEnv() error {
	commands := []string{"mv", "mkdir"}
	for _, command := range commands {
		if !channel.NewLocalChannel().IsCommandAvailable(command) {
			return fmt.Errorf("%s command not found", command)
		}
	}
	return nil
}
