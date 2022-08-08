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
	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

const StopProcessBin = "chaos_stopprocess"

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
			ActionExecutor: &StopProcessExecutor{},
			ActionExample: `
# Pause the process that contains the "SimpleHTTPServer" keyword
blade create process stop --process SimpleHTTPServer

# Pause the Java process
blade create process stop --process-cmd java

# Return success even if the process not found
blade create process stop --process demo --ignore-not-found`,
			ActionPrograms:   []string{StopProcessBin},
			ActionCategories: []string{category.SystemProcess},
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

func (s *StopProcessActionCommandSpec) LongDesc() string {
	if s.ActionLongDesc != "" {
		return s.ActionLongDesc
	}
	return "process fake death by process id or process name"
}

type StopProcessExecutor struct {
	channel spec.Channel
}

func (spe *StopProcessExecutor) Name() string {
	return "stop"
}

func (spe *StopProcessExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	resp := getPids(ctx, spe.channel, model, uid)
	if !resp.Success {
		return resp
	}
	pids := resp.Result.(string)
	if _, ok := spec.IsDestroy(ctx); ok {
		return spe.channel.Run(ctx, "kill", fmt.Sprintf("-CONT %s", pids))
	} else {
		return spe.channel.Run(ctx, "kill", fmt.Sprintf("-STOP %s", pids))
	}
}

func (spe *StopProcessExecutor) SetChannel(channel spec.Channel) {
	spe.channel = channel
}
