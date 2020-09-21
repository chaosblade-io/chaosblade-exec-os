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
	"regexp"
)

type FileChmodActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewFileChmodActionSpec() spec.ExpActionCommandSpec {
	return &FileChmodActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: fileCommFlags,
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "mark",
					Desc:     "--mark 777",
					Required: true,
				},
			},
			ActionExecutor: &FileChmodActionExecutor{},
			ActionExample:
`# Modify /home/logs/nginx.log file permissions to 777
blade create file chmod --filepath /home/logs/nginx.log --mark=777
`,
		},
	}
}

func (*FileChmodActionSpec) Name() string {
	return "chmod"
}

func (*FileChmodActionSpec) Aliases() []string {
	return []string{}
}

func (*FileChmodActionSpec) ShortDesc() string {
	return "File permission modification."
}

func (f *FileChmodActionSpec) LongDesc() string {
	return "File perçmission modification."
}

type FileChmodActionExecutor struct {
	channel spec.Channel
}

func (*FileChmodActionExecutor) Name() string {
	return "chmod"
}

var chmodFileBin = "chaos_chmodfile"

func (f *FileChmodActionExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	err := checkChmodFileExpEnv()
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

	if !util.IsExist(filepath) {
		return spec.ReturnFail(spec.Code[spec.IllegalParameters],
			fmt.Sprintf("the %s file does not exist", filepath))
	}

	mark := model.ActionFlags["mark"]
	match, _ := regexp.MatchString("^([0-7]{3})$", mark)
	if !match {
		return spec.ReturnFail(spec.Code[spec.IllegalParameters],
			fmt.Sprintf("the %s mark is fail", mark))
	}

	return f.start(filepath, mark, ctx)
}

func (f *FileChmodActionExecutor) start(filepath string, mark string, ctx context.Context) *spec.Response {
	flags := fmt.Sprintf(`--start --filepath "%s" --mark %s --debug=%t`, filepath, mark, util.Debug)
	return f.channel.Run(ctx, path.Join(f.channel.GetScriptPath(), chmodFileBin), flags)
}

func (f *FileChmodActionExecutor) stop(filepath string, ctx context.Context) *spec.Response {
	return f.channel.Run(ctx, path.Join(f.channel.GetScriptPath(), chmodFileBin),
		fmt.Sprintf(`--stop --filepath "%s" --debug=%t`, filepath, util.Debug))
}

func (f *FileChmodActionExecutor) SetChannel(channel spec.Channel) {
	f.channel = channel
}

func checkChmodFileExpEnv() error {
	commands := []string{"chmod", "grep", "echo", "rm", "awk"}
	for _, command := range commands {
		if !channel.NewLocalChannel().IsCommandAvailable(command) {
			return fmt.Errorf("%s command not found", command)
		}
	}
	return nil
}
