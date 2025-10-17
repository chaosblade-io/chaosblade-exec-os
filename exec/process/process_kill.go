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

package process

import (
	"context"
	"fmt"

	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"

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
				&spec.ExpFlag{
					Name: "pid",
					Desc: "pid",
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
blade create process kill --local-port 8080 --signal 15

# Return success even if the process not found
blade create process kill --process demo --ignore-not-found`,
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
	if _, ok := spec.IsDestroy(ctx); ok {
		return spec.ReturnSuccess(uid)
	}

	resp := getPids(ctx, kpe.channel, model, uid)
	if !resp.Success {
		return resp
	}
	pids := resp.Result.(string)
	signal := model.ActionFlags["signal"]
	if signal == "" {
		log.Errorf(ctx, "less signal flag value")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "signal")
	}
	return kpe.channel.Run(ctx, "kill", fmt.Sprintf("-%s %s", signal, pids))
}

func (kpe *KillProcessExecutor) SetChannel(channel spec.Channel) {
	kpe.channel = channel
}
