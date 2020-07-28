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
	"strconv"
)

type FileAppendActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewFileAppendActionSpec() spec.ExpActionCommandSpec {
	return &FileAppendActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "filepath",
					Desc:     "file path",
					Required: true,
				},
			},
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "content",
					Desc:     "append content",
					Required: true,
				},
				&spec.ExpFlag{
					Name: "count",
					Desc: "the number of append count, default 1",
				},
				&spec.ExpFlag{
					Name:   "interval",
					Desc:   "append interval, default 1s",
				},
				&spec.ExpFlag{
					Name:   "escape",
					Desc:   "symbols to escape, use --escape, at this --count is invalid",
					NoArgs: true,
				},
			},
			ActionExecutor: &FileAppendActionExecutor{},
		},
	}
}

func (*FileAppendActionSpec) Name() string {
	return "append"
}

func (*FileAppendActionSpec) Aliases() []string {
	return []string{}
}

func (*FileAppendActionSpec) ShortDesc() string {
	return "File content append"
}

func (f *FileAppendActionSpec) LongDesc() string {
	return "File content append. "
}

type FileAppendActionExecutor struct {
	channel spec.Channel
}

func (*FileAppendActionExecutor) Name() string {
	return "append"
}

var appeneFileBin = "chaos_appendfile"

func (f *FileAppendActionExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	err := checkAppendFileExpEnv()
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

	// default 1
	count := 1
	// 1000 ms
	interval := 1
	var escape bool

	content := model.ActionFlags["content"]
	countStr := model.ActionFlags["count"]
	intervalStr := model.ActionFlags["interval"]
	if countStr != "" {
		var err error
		count, err = strconv.Atoi(countStr)
		if err != nil || count < 1 {
			return spec.ReturnFail(spec.Code[spec.IllegalParameters], "--count value must be a positive integer")
		}
	}
	if intervalStr != "" {
		var err error
		interval, err = strconv.Atoi(intervalStr)
		if err != nil || interval < 1 {
			return spec.ReturnFail(spec.Code[spec.IllegalParameters], "--interval value must be a positive integer")
		}
	}

	escape = model.ActionFlags["escape"] == "true"

	if !util.IsExist(filepath) {
		return spec.ReturnFail(spec.Code[spec.IllegalParameters],
			fmt.Sprintf("the %s file does not exist", filepath))
	}

	return f.start(filepath, content, count, interval, escape, ctx)
}

func (f *FileAppendActionExecutor) start(filepath string, content string, count int, interval int, escape bool, ctx context.Context) *spec.Response {
	flags := fmt.Sprintf(`--start --filepath "%s" --content "%s" --count %d --interval %d --escape=%t --debug=%t`, filepath, content, count, interval, escape, util.Debug)
	return f.channel.Run(ctx, path.Join(f.channel.GetScriptPath(), appeneFileBin), flags)
}

func (f *FileAppendActionExecutor) stop(filepath string, ctx context.Context) *spec.Response {
	return f.channel.Run(ctx, path.Join(f.channel.GetScriptPath(), appeneFileBin),
		fmt.Sprintf(`--stop --filepath %s --debug=%t`, filepath, util.Debug))
}

func (f *FileAppendActionExecutor) SetChannel(channel spec.Channel) {
	f.channel = channel
}

func checkAppendFileExpEnv() error {
	commands := []string{"echo", "kill"}
	for _, command := range commands {
		if !channel.NewLocalChannel().IsCommandAvailable(command) {
			return fmt.Errorf("%s command not found", command)
		}
	}
	return nil
}
