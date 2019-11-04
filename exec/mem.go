/*
 * Copyright 1999-2019 Alibaba Group Holding Ltd.
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
	"github.com/chaosblade-io/chaosblade-spec-go/channel"
)

type MemCommandModelSpec struct {
	spec.BaseExpModelCommandSpec
}

func NewMemCommandModelSpec() spec.ExpModelCommandSpec {
	return &MemCommandModelSpec{
		spec.BaseExpModelCommandSpec{
			ExpActions: []spec.ExpActionCommandSpec{
				&loadActionCommand{
					spec.BaseExpActionCommandSpec{
						ActionMatchers: []spec.ExpFlagSpec{},
						ActionFlags:    []spec.ExpFlagSpec{},
						ActionExecutor: &memExecutor{},
					},
				},
			},
			ExpFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "mem-percent",
					Desc:     "percent of burn Memory (0-100)",
					Required: false,
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

type loadActionCommand struct {
	spec.BaseExpActionCommandSpec
}

func (*loadActionCommand) Name() string {
	return "load"
}

func (*loadActionCommand) Aliases() []string {
	return []string{}
}

func (*loadActionCommand) ShortDesc() string {
	return "mem load"
}

func (*loadActionCommand) LongDesc() string {
	return "mem load"
}

func (*loadActionCommand) Matchers() []spec.ExpFlagSpec {
	return []spec.ExpFlagSpec{}
}

func (*loadActionCommand) Flags() []spec.ExpFlagSpec {
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
	err := checkMemoryExpEnv()
	if err != nil {
		return spec.ReturnFail(spec.Code[spec.CommandNotFound], err.Error())
	}
	if ce.channel == nil {
		return spec.ReturnFail(spec.Code[spec.ServerError], "channel is nil")
	}
	if _, ok := spec.IsDestroy(ctx); ok {
		return ce.stop(ctx)
	}
	var memPercent int

	memPercentStr := model.ActionFlags["mem-percent"]
	if memPercentStr != "" {
		var err error
		memPercent, err = strconv.Atoi(memPercentStr)
		if err != nil {
			return spec.ReturnFail(spec.Code[spec.IllegalParameters],
				"--mem-percent value must be a positive integer")
		}
		if memPercent > 100 || memPercent < 0 {
			return spec.ReturnFail(spec.Code[spec.IllegalParameters],
				"--mem-percent value must be a prositive integer and not bigger than 100")
		}
	} else {
		memPercent = 100
	}

	return ce.start(ctx, memPercent)
}

const burnMemBin = "chaos_burnmem"

// start burn mem
func (ce *memExecutor) start(ctx context.Context, memPercent int) *spec.Response {
	args := fmt.Sprintf("--start --mem-percent %d", memPercent)
	return ce.channel.Run(ctx, path.Join(ce.channel.GetScriptPath(), burnMemBin), args)
}

// stop burn mem
func (ce *memExecutor) stop(ctx context.Context) *spec.Response {
	return ce.channel.Run(ctx, path.Join(ce.channel.GetScriptPath(), burnMemBin), "--stop")
}

func checkMemoryExpEnv() error {
	commands := []string{"ps", "awk", "grep", "kill", "nohup", "dd", "mount", "umount"}
	for _, command := range commands {
		if !channel.IsCommandAvailable(command) {
			return fmt.Errorf("%s command not found", command)
		}
	}
	return nil
}
