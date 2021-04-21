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

package deletefile

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/model"
	"path"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

// init registry provider to model.
func init() {
	model.Provide(new(DeleteFile))
}

type DeleteFile struct {
	Filepath        string `name:"filepath" json:"filepath" yaml:"filepath" default:"" help:"filepath"`
	Force           bool   `name:"force" json:"force" yaml:"force" default:"false" help:"force remove can't be restored"`
	AppendFileStart bool   `name:"start" json:"start" yaml:"start" default:"false" help:"start delete file"`
	AppendFileStop  bool   `name:"stop" json:"stop" yaml:"stop" default:"false" help:"stop delete file"`
	// default arguments
	Channel channel.OsChannel `kong:"-"`
	// for test mock
}

func (that *DeleteFile) Assign() model.Worker {
	return &DeleteFile{Channel: channel.NewLocalChannel()}
}

func (that *DeleteFile) Name() string {
	return exec.DeleteFileBin
}

func (that *DeleteFile) Exec() *spec.Response {
	if that.AppendFileStart {
		if that.Filepath == "" {
			bin.PrintErrAndExit("less --filepath flag")
		}
		that.startDeleteFile(that.Filepath, that.Force)
	} else if that.AppendFileStop {
		that.stopDeleteFile(that.Filepath, that.Force)
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
	return spec.ReturnSuccess("")
}

func (that *DeleteFile) startDeleteFile(filepath string, force bool) {
	ctx := context.Background()
	var response *spec.Response
	if force {
		response = that.Channel.Run(ctx, "rm", fmt.Sprintf(`-rf "%s"`, filepath))
		if !response.Success {
			bin.PrintErrAndExit(response.Err)
			return
		}
	} else {
		target := path.Join(path.Dir(filepath), "."+md5Hex(path.Base(filepath)))
		response = that.Channel.Run(ctx, "mv", fmt.Sprintf(`"%s" "%s"`, filepath, target))
		if !response.Success {
			bin.PrintErrAndExit(response.Err)
			return
		}
	}
	bin.PrintOutputAndExit(response.Result.(string))
}

func (that *DeleteFile) stopDeleteFile(filepath string, force bool) {
	if force {
		// nothing to do
	} else {
		ctx := context.Background()
		target := path.Join(path.Dir(filepath), "."+md5Hex(path.Base(filepath)))
		response := that.Channel.Run(ctx, "mv", fmt.Sprintf(`"%s" "%s"`, target, filepath))
		if !response.Success {
			bin.PrintErrAndExit(response.Err)
			return
		}
		bin.PrintOutputAndExit(response.Result.(string))
	}
}

func md5Hex(s string) string {
	m := md5.New()
	m.Write([]byte (s))
	return hex.EncodeToString(m.Sum(nil))
}
