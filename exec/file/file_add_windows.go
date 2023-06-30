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
	"encoding/base64"
	"fmt"
	"os"
	"path"

	"github.com/chaosblade-io/chaosblade-spec-go/log"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"

	"github.com/chaosblade-io/chaosblade-spec-go/spec"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
)

const AddFileBin = "chaos_addfile"

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
			ActionExample: `
# Create a file named nginx.log in the /home directory
blade create file add --filepath C:\nginx.log

# Create a file named nginx.log in the /home directory with the contents of HELLO WORLD
blade create file add --filepath C:\nginx.log --content "HELLO WORLD"

# Create a file named nginx.log in the /temp directory and automatically create directories that don't exist
blade create file add --filepath C:\nginx.log --auto-create-dir

# Create a directory named /nginx in the /temp directory and automatically create directories that don't exist
blade create file add --directory --filepath C:\nginx.log --auto-create-dir
`,
			ActionPrograms:   []string{AddFileBin},
			ActionCategories: []string{category.SystemFile},
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

func (f *FileAddActionExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	filepath := model.ActionFlags["filepath"]
	if _, ok := spec.IsDestroy(ctx); ok {
		return f.stop(filepath, ctx)
	}

	if exec.CheckFilepathExists(ctx, f.channel, filepath) {
		log.Errorf(ctx, "`%s`: filepath is exist", filepath)
		return spec.ResponseFailWithFlags(spec.ParameterInvalid, "filepath", filepath, "the filepath is exist")
	}

	directory := model.ActionFlags["directory"] == "true"
	content := model.ActionFlags["content"]
	enableBase64 := model.ActionFlags["enable-base64"] == "true"
	autoCreateDir := model.ActionFlags["auto-create-dir"] == "true"

	return f.start(f.channel, filepath, content, directory, enableBase64, autoCreateDir, ctx)
}

func (f *FileAddActionExecutor) start(cl spec.Channel, filepath, content string, directory, enableBase64, autoCreateDir bool, ctx context.Context) *spec.Response {

	dir := path.Dir(filepath)
	if autoCreateDir && !exec.CheckFilepathExists(ctx, cl, filepath) {
		if response := f.channel.Run(ctx, "mkdir", fmt.Sprintf(`%s`, dir)); !response.Success {
			return response
		}
	}
	if directory {
		return f.channel.Run(ctx, "mkdir", fmt.Sprintf(`%s`, filepath))
	} else {
		file, err := os.OpenFile(filepath, os.O_RDWR|os.O_CREATE, 0666)
		if err != nil {
			log.Errorf(ctx, err.Error())
			return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "create file", err.Error())
		}
		defer file.Close()

		if content == "" {
			return spec.Success()
		}

		if enableBase64 {
			if decodeBytes, err := base64.StdEncoding.DecodeString(content); err != nil {
				log.Errorf(ctx, err.Error())
				return spec.ResponseFailWithFlags(spec.ParameterInvalid, "content", content, err.Error())
			} else {
				content = string(decodeBytes)
			}
		}
		_, err = file.WriteString(content)
		if err != nil {
			log.Errorf(ctx, "write content failed, err: %s", err.Error())
			return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "write content", err.Error())
		}
		return spec.Success()
	}
}

func (f *FileAddActionExecutor) stop(filepath string, ctx context.Context) *spec.Response {
	return f.channel.Run(ctx, "del", fmt.Sprintf(`%s`, filepath))
}

func (f *FileAddActionExecutor) SetChannel(channel spec.Channel) {
	f.channel = channel
}
