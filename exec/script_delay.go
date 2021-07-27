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

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
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
			ActionExample: `
# Add commands to the script "start0() { sleep 10.000000 ...}"
blade create script delay --time 10000 --file test.sh --function-name start0`,
			ActionCategories: []string{category.SystemScript},
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

func (s *ScriptDelayActionCommand) LongDesc() string {
	if s.ActionLongDesc != "" {
		return s.ActionLongDesc
	}
	return "Sleep in script"
}

type ScriptDelayExecutor struct {
	channel spec.Channel
}

func (*ScriptDelayExecutor) Name() string {
	return "delay"
}

func (sde *ScriptDelayExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	commands := []string{"cat", "rm", "sed", "awk", "rm"}
	if response, ok := channel.NewLocalChannel().IsAllCommandsAvailable(commands); !ok {
		return response
	}
	if sde.channel == nil {
		util.Errorf(uid, util.GetRunFuncName(), spec.ChannelNil.Msg)
		return spec.ResponseFailWithFlags(spec.ChannelNil)
	}
	scriptFile := model.ActionFlags["file"]
	if scriptFile == "" {
		util.Errorf(uid, util.GetRunFuncName(), spec.ParameterLess.Sprintf("file"))
		return spec.ResponseFailWithFlags(spec.ParameterLess, "file")
	}
	if !util.IsExist(scriptFile) {
		util.Errorf(uid, util.GetRunFuncName(), fmt.Sprintf("`%s`, file is invalid. it not found", scriptFile))
		return spec.ResponseFailWithFlags(spec.ParameterInvalid, "file", scriptFile, "it is not found")
	}
	if _, ok := spec.IsDestroy(ctx); ok {
		return sde.stop(ctx, scriptFile)
	}
	functionName := model.ActionFlags["function-name"]
	if functionName == "" {
		util.Errorf(uid, util.GetRunFuncName(), spec.ParameterLess.Sprintf("function-name"))
		return spec.ResponseFailWithFlags(spec.ParameterLess, "function-name")
	}
	time := model.ActionFlags["time"]
	if time == "" {
		util.Errorf(uid, util.GetRunFuncName(), spec.ParameterLess.Sprintf("time"))
		return spec.ResponseFailWithFlags(spec.ParameterLess, "time")
	}
	t, err := strconv.Atoi(time)
	if err != nil {
		util.Errorf(uid, util.GetRunFuncName(), spec.ParameterIllegal.Sprintf("time", time, "it must be a positive integer"))
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "time", time, "ti must be a positive integer")
	}
	return sde.start(ctx, scriptFile, functionName, t)
}

func (sde *ScriptDelayExecutor) start(ctx context.Context, scriptFile, functionName string, timt int) *spec.Response {
	timeInSecond := float32(timt) / 1000.0
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
