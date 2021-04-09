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

package movefile

import (
	"context"
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
	model.Provide(new(MoveFile))
}

type MoveFile struct {
	Target          string `name:"target" json:"target" yaml:"target" default:"" help:"content"`
	Filepath        string `name:"filepath" json:"filepath" yaml:"filepath" default:"" help:"filepath"`
	Force           bool   `name:"force" json:"force" yaml:"force" default:"false" help:"overwrite target file"`
	AutoCreateDir   bool   `name:"auto-create-dir" json:"auto-create-dir" yaml:"auto-create-dir" default:"false" help:"automatically creates a directory that does not exist"`
	AppendFileStart bool   `name:"start" json:"start" yaml:"start" default:"false" help:"start move file"`
	AppendFileStop  bool   `name:"stop" json:"stop" yaml:"stop" default:"false" help:"stop move file"`
	// default arguments
	Channel channel.OsChannel `kong:"-"`
	// for test mock
}

func (that *MoveFile) Assign() model.Worker {
	return &MoveFile{Channel: channel.NewLocalChannel()}
}

func (that *MoveFile) Name() string {
	return exec.MoveFileBin
}

func (that *MoveFile) Exec() *spec.Response {
	if that.AppendFileStart {
		if that.Target == "" || that.Filepath == "" {
			bin.PrintErrAndExit("less --target or --filepath flag")
		}
		that.startMoveFile(that.Filepath, that.Target, that.Force, that.AutoCreateDir)
	} else if that.AppendFileStop {
		that.stopMoveFile(that.Filepath, that.Target)
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
	return spec.ReturnSuccess("")
}

func (that *MoveFile) startMoveFile(filepath, target string, force, autoCreateDir bool) {
	ctx := context.Background()
	var response *spec.Response

	if autoCreateDir && !util.IsExist(target) {
		response = that.Channel.Run(ctx, "mkdir", fmt.Sprintf(`-p %s`, target))
	}
	if !util.IsDir(target) {
		bin.PrintErrAndExit(fmt.Sprintf("the [%s] target file is not exists", target))
		return
	}
	if force {
		response = that.Channel.Run(ctx, "mv", fmt.Sprintf(`-f "%s" "%s"`, filepath, target))
	} else {
		response = that.Channel.Run(ctx, "mv", fmt.Sprintf(`"%s" "%s"`, filepath, target))
	}
	if !response.Success {
		bin.PrintErrAndExit(response.Err)
		return
	}
	bin.PrintOutputAndExit(response.Result.(string))
}

func (that *MoveFile) stopMoveFile(filepath, target string) {
	origin := path.Join(target, "/", path.Base(filepath))

	ctx := context.Background()
	response := that.Channel.Run(ctx, "mv", fmt.Sprintf(`-f "%s" "%s"`, origin, path.Dir(filepath)))
	if !response.Success {
		bin.PrintErrAndExit(response.Err)
		return
	}

	bin.PrintOutputAndExit(response.Result.(string))
}
