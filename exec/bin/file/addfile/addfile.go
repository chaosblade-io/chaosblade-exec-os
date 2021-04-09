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

package addfile

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/model"
	"path"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

// init registry provider to model.
func init() {
	model.Provide(new(AddFile))
}

type AddFile struct {
	Filepath        string `name:"filepath" json:"filepath" yaml:"filepath" default:"" help:"filepath"`
	Content         string `name:"content" json:"content" yaml:"content" default:"" help:"content"`
	Directory       bool   `name:"directory" json:"directory" yaml:"directory" default:"false" help:"is dir"`
	EnableBase64    bool   `name:"enable-base64" json:"enable-base64" yaml:"enable-base64" default:"false" help:"content support base64 encoding"`
	AutoCreateDir   bool   `name:"auto-create-dir" json:"auto-create-dir" yaml:"auto-create-dir" default:"false" help:"automatically creates a directory that does not exist"`
	AppendFileStart bool   `name:"start" json:"start" yaml:"start" default:"false" help:"start add file"`
	AppendFileStop  bool   `name:"stop" json:"stop" yaml:"stop" default:"false" help:"stop add file"`
	// default arguments
	Channel channel.OsChannel `kong:"-"`
	// for test mock
}

func (that *AddFile) Assign() model.Worker {
	return &AddFile{Channel: channel.NewLocalChannel()}
}

func (that *AddFile) Name() string {
	return exec.AddFileBin
}

func (that *AddFile) Exec() *spec.Response {
	if that.AppendFileStart {
		if that.Filepath == "" {
			bin.PrintErrAndExit("less --filepath flag")
		}
		that.startAddFile(that.Filepath, that.Content, that.Directory, that.EnableBase64, that.AutoCreateDir)
	} else if that.AppendFileStop {
		that.stopAddFile(that.Filepath)
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
	return spec.ReturnSuccess("")
}

func (that *AddFile) startAddFile(filepath, content string, directory, enableBase64, autoCreateDir bool) {
	ctx := context.Background()

	var response *spec.Response
	dir := path.Dir(filepath)
	if autoCreateDir && !util.IsExist(dir) {
		response = that.Channel.Run(ctx, "mkdir", fmt.Sprintf(`-p %s`, dir))
	}
	if directory {
		response = that.Channel.Run(ctx, "mkdir", fmt.Sprintf(`%s`, filepath))
	} else {
		if content == "" {
			response = that.Channel.Run(ctx, "touch", fmt.Sprintf(`%s`, filepath))
		} else {
			if enableBase64 {
				decodeBytes, err := base64.StdEncoding.DecodeString(content)
				if err != nil {
					bin.PrintErrAndExit(err.Error())
					return
				}
				content = string(decodeBytes)
			}
			response = that.Channel.Run(ctx, "echo", fmt.Sprintf(`"%s" >> "%s"`, content, filepath))
		}
	}
	if !response.Success {
		bin.PrintErrAndExit(response.Err)
		return
	}
	bin.PrintOutputAndExit(response.Result.(string))
}

func (that *AddFile) stopAddFile(filepath string) {

	ctx := context.Background()
	// get origin mark
	response := that.Channel.Run(ctx, "rm", fmt.Sprintf(`-rf %s`, filepath))
	if !response.Success {
		bin.PrintErrAndExit(response.Err)
		return
	}
	bin.PrintOutputAndExit(response.Result.(string))
}
