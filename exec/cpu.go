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
	"runtime"
	"strconv"
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
)

type CpuCommandModelSpec struct {
	spec.BaseExpModelCommandSpec
}

func NewCpuCommandModelSpec() spec.ExpModelCommandSpec {
	return &CpuCommandModelSpec{
		spec.BaseExpModelCommandSpec{
			ExpActions: []spec.ExpActionCommandSpec{
				&fullLoadActionCommand{
					spec.BaseExpActionCommandSpec{
						ActionMatchers: []spec.ExpFlagSpec{},
						ActionFlags:    []spec.ExpFlagSpec{},
						ActionExecutor: &cpuExecutor{},
					},
				},
			},
			ExpFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "cpu-count",
					Desc:     "Cpu count",
					Required: false,
				},
				&spec.ExpFlag{
					Name:     "cpu-list",
					Desc:     "CPUs in which to allow burning (0-3 or 1,3)",
					Required: false,
				},
				&spec.ExpFlag{
					Name:     "cpu-percent",
					Desc:     "percent of burn CPU (0-100)",
					Required: false,
				},
			},
		},
	}
}

func (*CpuCommandModelSpec) Name() string {
	return "cpu"
}

func (*CpuCommandModelSpec) ShortDesc() string {
	return "Cpu experiment"
}

func (*CpuCommandModelSpec) LongDesc() string {
	return "Cpu experiment, for example full load"
}

func (*CpuCommandModelSpec) Example() string {
	return "blade create cpu load --cpu-percent 80"
}

type fullLoadActionCommand struct {
	spec.BaseExpActionCommandSpec
}

func (*fullLoadActionCommand) Name() string {
	return "fullload"
}

func (*fullLoadActionCommand) Aliases() []string {
	return []string{"fl", "load"}
}

func (*fullLoadActionCommand) ShortDesc() string {
	return "cpu load"
}

func (*fullLoadActionCommand) LongDesc() string {
	return "cpu load"
}

func (*fullLoadActionCommand) Matchers() []spec.ExpFlagSpec {
	return []spec.ExpFlagSpec{}
}

func (*fullLoadActionCommand) Flags() []spec.ExpFlagSpec {
	return []spec.ExpFlagSpec{}
}

type cpuExecutor struct {
	channel spec.Channel
}

func (ce *cpuExecutor) Name() string {
	return "cpu"
}

func (ce *cpuExecutor) SetChannel(channel spec.Channel) {
	ce.channel = channel
}

func (ce *cpuExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	err := checkCpuExpEnv()
	if err != nil {
		return spec.ReturnFail(spec.Code[spec.CommandNotFound], err.Error())
	}
	if ce.channel == nil {
		return spec.ReturnFail(spec.Code[spec.ServerError], "channel is nil")
	}
	if _, ok := spec.IsDestroy(ctx); ok {
		return ce.stop(ctx)
	}
	var cpuCount int
	var cpuList string
	var cpuPercent int

	cpuPercentStr := model.ActionFlags["cpu-percent"]
	if cpuPercentStr != "" {
		var err error
		cpuPercent, err = strconv.Atoi(cpuPercentStr)
		if err != nil {
			return spec.ReturnFail(spec.Code[spec.IllegalParameters],
				"--cpu-percent value must be a positive integer")
		}
		if cpuPercent > 100 || cpuPercent < 0 {
			return spec.ReturnFail(spec.Code[spec.IllegalParameters],
				"--cpu-percent value must be a prositive integer and not bigger than 100")
		}
	} else {
		cpuPercent = 100
	}

	cpuListStr := model.ActionFlags["cpu-list"]
	if cpuListStr != "" {
		if !channel.NewLocalChannel().IsCommandAvailable("taskset") {
			return spec.ReturnFail(spec.Code[spec.EnvironmentError],
				"taskset command not exist")
		}
		cores, err := util.ParseIntegerListToStringSlice(cpuListStr)
		if err != nil {
			return spec.ReturnFail(spec.Code[spec.IllegalParameters],
				fmt.Sprintf("parse %s flag err, %v", "cpu-list", err))
		}
		cpuList = strings.Join(cores, ",")
	} else {
		// if cpu-list value is not empty, then the cpu-count flag is invalid
		var err error
		cpuCountStr := model.ActionFlags["cpu-count"]
		if cpuCountStr != "" {
			cpuCount, err = strconv.Atoi(cpuCountStr)
			if err != nil {
				return spec.ReturnFail(spec.Code[spec.IllegalParameters],
					"--cpu-count value must be a positive integer")
			}
		}
		if cpuCount <= 0 || int(cpuCount) > runtime.NumCPU() {
			cpuCount = runtime.NumCPU()
		}
	}
	return ce.start(ctx, cpuList, cpuCount, cpuPercent)
}

const burnCpuBin = "chaos_burncpu"

// start burn cpu
func (ce *cpuExecutor) start(ctx context.Context, cpuList string, cpuCount int, cpuPercent int) *spec.Response {
	args := fmt.Sprintf("--start --cpu-count %d --cpu-percent %d --debug=%t", cpuCount, cpuPercent, util.Debug)
	if cpuList != "" {
		args = fmt.Sprintf("%s --cpu-list %s", args, cpuList)
	}
	return ce.channel.Run(ctx, path.Join(ce.channel.GetScriptPath(), burnCpuBin), args)
}

// stop burn cpu
func (ce *cpuExecutor) stop(ctx context.Context) *spec.Response {
	return ce.channel.Run(ctx, path.Join(ce.channel.GetScriptPath(), burnCpuBin),
		fmt.Sprintf("--stop --debug=%t", util.Debug))
}

func checkCpuExpEnv() error {
	commands := []string{"ps", "awk", "grep", "kill", "nohup", "tr"}
	for _, command := range commands {
		if !channel.NewLocalChannel().IsCommandAvailable(command) {
			return fmt.Errorf("%s command not found", command)
		}
	}
	return nil
}
