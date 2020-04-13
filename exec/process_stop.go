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

	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
)

type StopProcessActionCommandSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewStopProcessActionCommandSpec() spec.ExpActionCommandSpec {
	return &StopProcessActionCommandSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name: "process",
					Desc: "Process name",
				},
				&spec.ExpFlag{
					Name: "process-cmd",
					Desc: "Process name in command",
				},
			},
			ActionFlags:    []spec.ExpFlagSpec{},
			ActionExecutor: &StopProcessExecutor{},
		},
	}
}

func (*StopProcessActionCommandSpec) Name() string {
	return "stop"
}

func (*StopProcessActionCommandSpec) Aliases() []string {
	return []string{"f"}
}

func (*StopProcessActionCommandSpec) ShortDesc() string {
	return "process fake death"
}

func (*StopProcessActionCommandSpec) LongDesc() string {
	return "process fake death by process id or process name"
}

type StopProcessExecutor struct {
	channel spec.Channel
}

func (spe *StopProcessExecutor) Name() string {
	return "stop"
}

var stopProcessBin = "chaos_stopprocess"

func (spe *StopProcessExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	err := checkProcessExpEnv()
	if err != nil {
		return spec.ReturnFail(spec.Code[spec.CommandNotFound], err.Error())
	}
	if spe.channel == nil {
		return spec.ReturnFail(spec.Code[spec.ServerError], "channel is nil")
	}
	process := model.ActionFlags["process"]
	processCmd := model.ActionFlags["process-cmd"]
	if process == "" && processCmd == "" {
		return spec.ReturnFail(spec.Code[spec.IllegalParameters], "less process matcher")
	}
	flags := ""
	if process != "" {
		flags = fmt.Sprintf(`--process "%s"`, process)
	} else if processCmd != "" {
		flags = fmt.Sprintf(`--process-cmd "%s"`, processCmd)
	}

	if _, ok := spec.IsDestroy(ctx); ok {
		return spe.recoverProcess(flags, ctx)
	} else {
		return spe.stopProcess(flags, ctx)
	}
}

func (spe *StopProcessExecutor) stopProcess(flags string, ctx context.Context) *spec.Response {
	args := fmt.Sprintf("--start --debug=%t", util.Debug)
	flags = fmt.Sprintf("%s %s", args, flags)
	return spe.channel.Run(ctx, path.Join(spe.channel.GetScriptPath(), stopProcessBin), flags)
}

func (spe *StopProcessExecutor) recoverProcess(flags string, ctx context.Context) *spec.Response {
	args := fmt.Sprintf("--stop --debug=%t", util.Debug)
	flags = fmt.Sprintf("%s %s", args, flags)
	return spe.channel.Run(ctx, path.Join(spe.channel.GetScriptPath(), stopProcessBin), flags)
}

func (spe *StopProcessExecutor) SetChannel(channel spec.Channel) {
	spe.channel = channel
}
