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

package chmodfile

import (
	"context"
	"fmt"
	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/model"
	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

// init registry provider to model.
func init() {
	model.Provide(new(ChmodFile))
}

type ChmodFile struct {
	Filepath        string `name:"filepath" json:"filepath" yaml:"filepath" default:"" help:"filepath"`
	Mark            string `name:"mark" json:"mark" yaml:"mark" default:"" help:"content"`
	AppendFileStart bool   `name:"start" json:"start" yaml:"start" default:"false" help:"start change modify file"`
	AppendFileStop  bool   `name:"stop" json:"stop" yaml:"stop" default:"false" help:"stop change modify file"`
	// default arguments
	Channel channel.OsChannel `kong:"-"`
	// for test mock
}

func (that *ChmodFile) Assign() model.Worker {
	return &ChmodFile{Channel: channel.NewLocalChannel()}
}

func (that *ChmodFile) Name() string {
	return exec.ChmodFileBin
}

func (that *ChmodFile) Exec() *spec.Response {
	if that.AppendFileStart {
		if that.Mark == "" || that.Filepath == "" {
			bin.PrintErrAndExit("less --mark or --filepath flag")
		}
		that.startChmodFile(that.Filepath, that.Mark)
	} else if that.AppendFileStop {
		that.stopChmodFile(that.Filepath)
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
	return spec.ReturnSuccess("")
}

const tmpFileChmod = "/tmp/chaos-file-chmod.tmp"

func (that *ChmodFile) startChmodFile(filepath, mark string) {
	ctx := context.Background()

	response := that.Channel.Run(ctx, "grep", fmt.Sprintf(`-q "%s:" "%s"`, filepath, tmpFileChmod))
	if response.Success {
		bin.PrintErrAndExit(fmt.Sprintf("%s is already being experimented o", filepath))
		return
	}

	fileInfo, _ := os.Stat(filepath)
	originMark := strconv.FormatInt(int64(fileInfo.Mode().Perm()), 8)

	response = that.Channel.Run(ctx, "echo", fmt.Sprintf(`%s:%s >> %s`, filepath, originMark, tmpFileChmod))
	if !response.Success {
		bin.PrintErrAndExit(response.Err)
		return
	}
	response = that.Channel.Run(ctx, "chmod", fmt.Sprintf(`%s "%s"`, mark, filepath))
	if !response.Success {
		bin.PrintErrAndExit(response.Err)
		return
	}
	bin.PrintOutputAndExit(response.Result.(string))
}

func (that *ChmodFile) stopChmodFile(filepath string) {

	ctx := context.Background()
	// get origin mark
	response := that.Channel.Run(ctx, "grep", fmt.Sprintf(`%s: %s | awk -F ':' '{printf $2}'`, filepath, tmpFileChmod))
	if !response.Success {
		that.clearTempFile(filepath, response, ctx)
		bin.PrintErrAndExit(response.Err)
		return
	}

	originMark := response.Result.(string)
	match, _ := regexp.MatchString("^([0-7]{3})$", originMark)
	if !match {
		bin.PrintErrAndExit(fmt.Sprintf("the %s mark is fail", that.Mark))
		return
	}

	response = that.Channel.Run(ctx, "chmod", fmt.Sprintf(`%s %s`, originMark, filepath))
	if !response.Success {
		that.clearTempFile(filepath, response, ctx)
		bin.PrintErrAndExit(response.Err)
		return
	}
	response, done := that.clearTempFile(filepath, response, ctx)
	if done {
		return
	}

	bin.PrintOutputAndExit(originMark)
}

func (that *ChmodFile) clearTempFile(filepath string, response *spec.Response, ctx context.Context) (*spec.Response, bool) {

	response = that.Channel.Run(ctx, "cat", fmt.Sprintf(`"%s"| grep -v %s:`, tmpFileChmod, filepath))
	if !response.Success {
		response = that.Channel.Run(ctx, "rm", fmt.Sprintf(`-rf "%s"`, tmpFileChmod))
		if !response.Success {
			bin.PrintErrAndExit(response.Err)
			return nil, true
		}
	} else {
		response = that.Channel.Run(ctx, "echo", fmt.Sprintf(`"%s" > %s`,
			strings.TrimRight(response.Result.(string), "\n"),
			tmpFileChmod))

		if !response.Success {
			bin.PrintErrAndExit(response.Err)
			return nil, true
		}
	}
	return response, false
}
