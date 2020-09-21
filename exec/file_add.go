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
	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
	"path"
)

type FileAddActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewFileAddActionSpec() spec.ExpActionCommandSpec {
	return &FileAddActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: fileCommFlags,
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:   "directory",
					Desc:   "use --directory flag, --filepath is directory",
					NoArgs: true,
				},
				&spec.ExpFlag{
					Name: "content",
					Desc: "--content, add file content",
				},
				&spec.ExpFlag{
					Name:   "enable-base64",
					Desc:   "--content use base64 encoding",
					NoArgs: true,
				},
				&spec.ExpFlag{
					Name:   "auto-create-dir",
					Desc:   "automatically creates a directory that does not exist",
					NoArgs: true,
				},
			},
			ActionExecutor: &FileAddActionExecutor{},
			ActionExample:
`# Create a file named nginx.log in the /home directory
blade create file add --filepath /home/nginx.log

# Create a file named nginx.log in the /home directory with the contents of HELLO WORLD
blade create file add --filepath /home/nginx.log --content "HELLO WORLD"

# Create a file named nginx.log in the /temp directory and automatically create directories that don't exist
blade create file add --filepath /temp/nginx.log --auto-create-dir

# Create a directory named /nginx in the /temp directory and automatically create directories that don't exist
blade create file add --directory --filepath /temp/nginx --auto-create-dir
`,
		},
	}
}

func (*FileAddActionSpec) Name() string {
	return "add"
}

func (*FileAddActionSpec) Aliases() []string {
	return []string{}
}

func (*FileAddActionSpec) ShortDesc() string {
	return "File or path add"
}

func (f *FileAddActionSpec) LongDesc() string {
	return "File or path add"
}

type FileAddActionExecutor struct {
	channel spec.Channel
}

func (*FileAddActionExecutor) Name() string {
	return "add"
}

var addFileBin = "chaos_addfile"

func (f *FileAddActionExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	err := checkAddFileExpEnv()
	if err != nil {
		return spec.ReturnFail(spec.Code[spec.CommandNotFound], err.Error())
	}

	if f.channel == nil {
		return spec.ReturnFail(spec.Code[spec.ServerError], "channel is nil")
	}

	filepath := model.ActionFlags["filepath"]
	if _, ok := spec.IsDestroy(ctx); ok {
		return f.stop(filepath, ctx)
	}

	if util.IsExist(filepath) {
		return spec.ReturnFail(spec.Code[spec.IllegalParameters],
			fmt.Sprintf("the %s file is exist", filepath))
	}

	directory := model.ActionFlags["directory"] == "true"
	content := model.ActionFlags["content"]
	enableBase64 := model.ActionFlags["enable-base64"] == "true"
	autoCreateDir := model.ActionFlags["auto-create-dir"] == "true"

	return f.start(filepath, content, directory, enableBase64, autoCreateDir, ctx)
}

func (f *FileAddActionExecutor) start(filepath, content string, directory, enableBase64, autoCreateDir bool, ctx context.Context) *spec.Response {
	flags := fmt.Sprintf(`--start --filepath "%s" --content "%s" --debug=%t`, filepath, content, util.Debug)
	if directory {
		flags = fmt.Sprintf(`%s --directory=true`, flags)
	}
	if enableBase64 {
		flags = fmt.Sprintf(`%s --enable-base64=true`, flags)
	}
	if autoCreateDir {
		flags = fmt.Sprintf(`%s --auto-create-dir=true`, flags)
	}
	return f.channel.Run(ctx, path.Join(f.channel.GetScriptPath(), addFileBin), flags)
}

func (f *FileAddActionExecutor) stop(filepath string, ctx context.Context) *spec.Response {
	return f.channel.Run(ctx, path.Join(f.channel.GetScriptPath(), addFileBin),
		fmt.Sprintf(`--stop --filepath %s --debug=%t`, filepath, util.Debug))
}

func (f *FileAddActionExecutor) SetChannel(channel spec.Channel) {
	f.channel = channel
}

func checkAddFileExpEnv() error {
	commands := []string{"touch", "mkdir", "echo", "rm"}
	for _, command := range commands {
		if !channel.NewLocalChannel().IsCommandAvailable(command) {
			return fmt.Errorf("%s command not found", command)
		}
	}
	return nil
}
