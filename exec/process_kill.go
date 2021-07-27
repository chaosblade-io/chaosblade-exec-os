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

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
)

const KillProcessBin = "chaos_killprocess"

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
			ActionExample: `
# Kill the process that contains the SimpleHTTPServer keyword
blade create process kill --process SimpleHTTPServer

# Kill the Java process
blade create process kill --process-cmd java

# Specifies the semaphore and local port to kill the process
blade c process kill --local-port 8080 --signal 15

# Return success even if the process not found
blade c process kill --process demo --ignore-not-found`,
			ActionPrograms:   []string{KillProcessBin},
			ActionCategories: []string{category.SystemProcess},
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

func (k *KillProcessActionCommandSpec) LongDesc() string {
	if k.ActionLongDesc != "" {
		return k.ActionLongDesc
	}
	return "Kill process by process id or process name"
}

func (*KillProcessActionCommandSpec) Categories() []string {
	return []string{category.SystemProcess}
}

type KillProcessExecutor struct {
	channel spec.Channel
}

func (kpe *KillProcessExecutor) Name() string {
	return "kill"
}

func (kpe *KillProcessExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	if kpe.channel == nil {
		util.Errorf(uid, util.GetRunFuncName(), spec.ChannelNil.Msg)
		return spec.ResponseFailWithFlags(spec.ChannelNil)
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
	ignoreProcessNotFound := model.ActionFlags["ignore-not-found"] == "true"
	if process == "" && processCmd == "" && localPorts == "" {
		util.Errorf(uid, util.GetRunFuncName(), "less process、process-cmd and local-port, less process matcher")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "process|process-cmd|local-port")
	}

	var excludeProcessValue = fmt.Sprintf("blade,%s", excludeProcess)
	ctx = context.WithValue(ctx, channel.ExcludeProcessKey, excludeProcessValue)
	if !ignoreProcessNotFound {
		if response := checkProcessInvalid(uid, process, processCmd, localPorts, ctx); response != nil {
			return response
		}
	}
	flags := fmt.Sprintf("--debug=%t", util.Debug)
	if countValue != "" {
		count, err := strconv.Atoi(countValue)
		if err != nil {
			util.Errorf(uid, util.GetRunFuncName(), spec.ParameterIllegal.Sprintf("count", countValue, err))
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "count", countValue, err)
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
	if ignoreProcessNotFound {
		flags = fmt.Sprintf(`%s --ignore-not-found=%t`, flags, ignoreProcessNotFound)
	}
	return kpe.channel.Run(ctx, path.Join(kpe.channel.GetScriptPath(), KillProcessBin), flags)
}

func (kpe *KillProcessExecutor) SetChannel(channel spec.Channel) {
	kpe.channel = channel
}
