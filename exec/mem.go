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

const BurnMemBin = "chaos_burnmem"

type MemCommandModelSpec struct {
	spec.BaseExpModelCommandSpec
}

func NewMemCommandModelSpec() spec.ExpModelCommandSpec {
	return &MemCommandModelSpec{
		spec.BaseExpModelCommandSpec{
			ExpActions: []spec.ExpActionCommandSpec{
				&MemLoadActionCommand{
					spec.BaseExpActionCommandSpec{
						ActionMatchers: []spec.ExpFlagSpec{},
						ActionFlags:    []spec.ExpFlagSpec{},
						ActionExecutor: &memExecutor{},
						ActionExample: `
# The execution memory footprint is 50%
blade create mem load --mode ram --mem-percent 50

# The execution memory footprint is 50%, cache model
blade create mem load --mode cache --mem-percent 50

# The execution memory footprint is 50%, usage contains buffer/cache
blade create mem load --mode ram --mem-percent 50 --include-buffer-cache

# The execution memory footprint is 50%, avoid mem-burn process being killed
blade create mem load --mode ram --mem-percent 50 --avoid-being-killed

# The execution memory footprint is 50% for 200 seconds
blade create mem load --mode ram --mem-percent 50 --timeout 200

# 200M memory is reserved
blade create mem load --mode ram --reserve 200 --rate 100`,
						ActionPrograms:   []string{BurnMemBin},
						ActionCategories: []string{category.SystemMem},
					},
				},
			},
			ExpFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "mem-percent",
					Desc:     "percent of burn Memory (0-100), must be a positive integer",
					Required: false,
				},
				&spec.ExpFlag{
					Name:     "reserve",
					Desc:     "reserve to burn Memory, unit is MB. If the mem-percent flag exist, use mem-percent first.",
					Required: false,
				},
				&spec.ExpFlag{
					Name:     "rate",
					Desc:     "burn memory rate, unit is M/S, only support for ram mode.",
					Required: false,
				},
				&spec.ExpFlag{
					Name:     "mode",
					Desc:     "burn memory mode, cache or ram.",
					Required: false,
				},
				&spec.ExpFlag{
					Name:   "include-buffer-cache",
					Desc:   "Ram mode mem-percent is include buffer/cache",
					NoArgs: true,
				},
				&spec.ExpFlag{
					Name:   "avoid-being-killed",
					Desc:   "Prevent mem-burn process from being killed by oom-killer",
					NoArgs: true,
				},
			},
		},
	}
}

func (*MemCommandModelSpec) Name() string {
	return "mem"
}

func (*MemCommandModelSpec) ShortDesc() string {
	return "Mem experiment"
}

func (*MemCommandModelSpec) LongDesc() string {
	return "Mem experiment, for example load"
}

func (*MemCommandModelSpec) Example() string {
	return "mem load"
}

type MemLoadActionCommand struct {
	spec.BaseExpActionCommandSpec
}

func (*MemLoadActionCommand) Name() string {
	return "load"
}

func (*MemLoadActionCommand) Aliases() []string {
	return []string{}
}

func (*MemLoadActionCommand) ShortDesc() string {
	return "mem load"
}

func (l *MemLoadActionCommand) LongDesc() string {
	if l.ActionLongDesc != "" {
		return l.ActionLongDesc
	}
	return "Create chaos engineering experiments with memory load"
}

func (*MemLoadActionCommand) Matchers() []spec.ExpFlagSpec {
	return []spec.ExpFlagSpec{}
}

func (*MemLoadActionCommand) Flags() []spec.ExpFlagSpec {
	return []spec.ExpFlagSpec{}
}

type memExecutor struct {
	channel spec.Channel
}

func (ce *memExecutor) Name() string {
	return "mem"
}

func (ce *memExecutor) SetChannel(channel spec.Channel) {
	ce.channel = channel
}

func (ce *memExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	commands := []string{"dd", "mount", "umount"}
	if response, ok := channel.NewLocalChannel().IsAllCommandsAvailable(commands); !ok {
		return response
	}

	if ce.channel == nil {
		util.Errorf(uid, util.GetRunFuncName(), spec.ChannelNil.Msg)
		return spec.ResponseFailWithFlags(spec.ChannelNil)
	}
	if _, ok := spec.IsDestroy(ctx); ok {
		return ce.stop(ctx, model.ActionFlags["mode"])
	}
	var memPercent, memReserve, memRate int

	memPercentStr := model.ActionFlags["mem-percent"]
	memReserveStr := model.ActionFlags["reserve"]
	memRateStr := model.ActionFlags["rate"]
	burnMemModeStr := model.ActionFlags["mode"]
	includeBufferCache := model.ActionFlags["include-buffer-cache"] == "true"
	avoidBeingKilled := model.ActionFlags["avoid-being-killed"] == "true"

	var err error
	if memPercentStr != "" {
		var err error
		memPercent, err = strconv.Atoi(memPercentStr)
		if err != nil {
			util.Errorf(uid, util.GetRunFuncName(), fmt.Sprintf("`%s`: mem-percent  must be a positive integer", memPercentStr))
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "mem-percent", memPercentStr, "it must be a positive integer")
		}
		if memPercent > 100 || memPercent < 0 {
			util.Errorf(uid, util.GetRunFuncName(), fmt.Sprintf("`%s`: mem-percent  must be a positive integer and not bigger than 100", memPercentStr))
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "mem-percent", memPercentStr, "it must be a positive integer and not bigger than 100")
		}
	} else if memReserveStr != "" {
		memReserve, err = strconv.Atoi(memReserveStr)
		if err != nil {
			util.Errorf(uid, util.GetRunFuncName(), fmt.Sprintf("`%s`: reserve  must be a positive integer", memReserveStr))
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "reserve", memReserveStr, err)
		}
	} else {
		memPercent = 100
	}
	if memRateStr != "" {
		memRate, err = strconv.Atoi(memRateStr)
		if err != nil {
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "rate", memRateStr, "it must be a positive integer")
		}
	}
	return ce.start(ctx, memPercent, memReserve, memRate, burnMemModeStr, includeBufferCache, avoidBeingKilled)
}

// start burn mem
func (ce *memExecutor) start(ctx context.Context, memPercent, memReserve, memRate int, burnMemMode string, includeBufferCache bool, avoidBeingKilled bool) *spec.Response {
	args := fmt.Sprintf("--start --mem-percent %d --reserve %d --debug=%t", memPercent, memReserve, util.Debug)
	if memRate != 0 {
		args = fmt.Sprintf("%s --rate %d", args, memRate)
	}
	if burnMemMode != "" {
		args = fmt.Sprintf("%s --mode %s", args, burnMemMode)
	}
	if includeBufferCache {
		args = fmt.Sprintf("%s --include-buffer-cache=%t", args, includeBufferCache)
	}
	if avoidBeingKilled {
		args = fmt.Sprintf("%s --avoid-being-killed=%t", args, avoidBeingKilled)
	}
	return ce.channel.Run(ctx, path.Join(ce.channel.GetScriptPath(), BurnMemBin), args)
}

// stop burn mem
func (ce *memExecutor) stop(ctx context.Context, burnMemMode string) *spec.Response {
	args := fmt.Sprintf("--stop --debug=%t", util.Debug)
	if burnMemMode != "" {
		args = fmt.Sprintf("%s --mode %s", args, burnMemMode)
	}
	return ce.channel.Run(ctx, path.Join(ce.channel.GetScriptPath(), BurnMemBin), args)
}
