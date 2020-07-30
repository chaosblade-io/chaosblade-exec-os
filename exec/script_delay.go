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
	"strconv"

	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
)

type ScriptDelayActionCommand struct {
	spec.BaseExpActionCommandSpec
}

func NewScriptDelayActionCommand() spec.ExpActionCommandSpec {
	return &ScriptDelayActionCommand{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{},
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "time",
					Desc:     "sleep time, unit is millisecond",
					Required: true,
				},
			},
			ActionExecutor: &ScriptDelayExecutor{},
		},
	}
}

func (*ScriptDelayActionCommand) Name() string {
	return "delay"
}

func (*ScriptDelayActionCommand) Aliases() []string {
	return []string{}
}

func (*ScriptDelayActionCommand) ShortDesc() string {
	return "Script executed delay"
}

func (*ScriptDelayActionCommand) LongDesc() string {
	return "Sleep in script"
}

type ScriptDelayExecutor struct {
	channel spec.Channel
}

func (*ScriptDelayExecutor) Name() string {
	return "delay"
}

func (sde *ScriptDelayExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	err := checkScriptExpEnv()
	if err != nil {
		return spec.ReturnFail(spec.Code[spec.CommandNotFound], err.Error())
	}
	if sde.channel == nil {
		return spec.ReturnFail(spec.Code[spec.ServerError], "channel is nil")
	}
	scriptFile := model.ActionFlags["file"]
	if scriptFile == "" {
		return spec.ReturnFail(spec.Code[spec.IllegalParameters], "must specify --file flag")
	}
	if !util.IsExist(scriptFile) {
		return spec.ReturnFail(spec.Code[spec.FileNotFound],
			fmt.Sprintf("%s file not found", scriptFile))
	}
	if _, ok := spec.IsDestroy(ctx); ok {
		return sde.stop(ctx, scriptFile)
	}
	functionName := model.ActionFlags["function-name"]
	if functionName == "" {
		return spec.ReturnFail(spec.Code[spec.IllegalParameters], "must specify --function-name flag")
	}
	time := model.ActionFlags["time"]
	if time == "" {
		return spec.ReturnFail(spec.Code[spec.IllegalParameters], "must specify --time flag")
	}
	return sde.start(ctx, scriptFile, functionName, time)
}

func (sde *ScriptDelayExecutor) start(ctx context.Context, scriptFile, functionName, time string) *spec.Response {
	t, err := strconv.Atoi(time)
	if err != nil {
		return spec.ReturnFail(spec.Code[spec.IllegalParameters], "time must be positive integer")
	}
	timeInSecond := float32(t) / 1000.0
	// backup file
	response := backScript(sde.channel, scriptFile)
	if !response.Success {
		return response
	}
	response = insertContentToScriptBy(sde.channel, functionName, fmt.Sprintf("sleep %f", timeInSecond), scriptFile)
	if !response.Success {
		sde.stop(ctx, scriptFile)
	}
	return response
}

func (sde *ScriptDelayExecutor) stop(ctx context.Context, scriptFile string) *spec.Response {
	return recoverScript(sde.channel, scriptFile)
}

func (sde *ScriptDelayExecutor) SetChannel(channel spec.Channel) {
	sde.channel = channel
}
