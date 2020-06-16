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
	"strconv"

	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
)

type KillProcessActionCommandSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewKillProcessActionCommandSpec() spec.ExpActionCommandSpec {
	return &KillProcessActionCommandSpec{
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
				&spec.ExpFlag{
					Name: "count",
					Desc: "Limit count, 0 means unlimited",
				},
				&spec.ExpFlag{
					Name: "local-port",
					Desc: "Local service ports. Separate multiple ports with commas (,) or connector representing ranges, for example: 80,8000-8080",
				},
				&spec.ExpFlag{
					Name: "signal",
					Desc: "Killing process signal, such as 9,15",
				},
				&spec.ExpFlag{
					Name: "exclude-process",
					Desc: "Exclude process",
				},
			},
			ActionFlags:    []spec.ExpFlagSpec{},
			ActionExecutor: &KillProcessExecutor{},
		},
	}
}

func (*KillProcessActionCommandSpec) Name() string {
	return "kill"
}

func (*KillProcessActionCommandSpec) Aliases() []string {
	return []string{"k"}
}

func (*KillProcessActionCommandSpec) ShortDesc() string {
	return "Kill process"
}

func (*KillProcessActionCommandSpec) LongDesc() string {
	return "Kill process by process id or process name"
}

type KillProcessExecutor struct {
	channel spec.Channel
}

func (kpe *KillProcessExecutor) Name() string {
	return "kill"
}

var killProcessBin = "chaos_killprocess"

func (kpe *KillProcessExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	err := checkProcessExpEnv()
	if err != nil {
		return spec.ReturnFail(spec.Code[spec.CommandNotFound], err.Error())
	}
	if kpe.channel == nil {
		return spec.ReturnFail(spec.Code[spec.ServerError], "channel is nil")
	}
	if _, ok := spec.IsDestroy(ctx); ok {
		return spec.ReturnSuccess(uid)
	}
	countValue := model.ActionFlags["count"]
	process := model.ActionFlags["process"]
	processCmd := model.ActionFlags["process-cmd"]
	localPorts := model.ActionFlags["local-port"]
	signal := model.ActionFlags["signal"]
	excludeProcess := model.ActionFlags["exclude-process"]
	if process == "" && processCmd == "" && localPorts == "" {
		return spec.ReturnFail(spec.Code[spec.IllegalParameters], "less process matcher")
	}
	flags := fmt.Sprintf("--debug=%t", util.Debug)
	if countValue != "" {
		count, err := strconv.Atoi(countValue)
		if err != nil {
			return spec.ReturnFail(spec.Code[spec.IllegalParameters], err.Error())
		}
		flags = fmt.Sprintf("%s --count %d", flags, count)
	}
	if process != "" {
		flags = fmt.Sprintf(`%s --process "%s"`, flags, process)
	} else if processCmd != "" {
		flags = fmt.Sprintf(`%s --process-cmd "%s"`, flags, processCmd)
	} else if localPorts != "" {
		flags = fmt.Sprintf(`%s --local-port "%s"`, flags, localPorts)
	}
	if signal != "" {
		flags = fmt.Sprintf(`%s --signal %s`, flags, signal)
	}
	if excludeProcess != "" {
		flags = fmt.Sprintf(`%s --exclude-process %s`, flags, excludeProcess)
	}
	return kpe.channel.Run(ctx, path.Join(kpe.channel.GetScriptPath(), killProcessBin), flags)
}

func (kpe *KillProcessExecutor) SetChannel(channel spec.Channel) {
	kpe.channel = channel
}
