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

package script

import (
	"context"
	"fmt"
	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"strconv"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
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
	if response, ok := sde.channel.IsAllCommandsAvailable(ctx, commands); !ok {
		return response
	}

	scriptFile := model.ActionFlags["file"]
	if scriptFile == "" {
		log.Errorf(ctx, "file is nil")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "file")
	}
	if !exec.CheckFilepathExists(ctx, sde.channel, scriptFile) {
		log.Errorf(ctx, "`%s`, file is invalid. it not found", scriptFile)
		return spec.ResponseFailWithFlags(spec.ParameterInvalid, "file", scriptFile, "it is not found")
	}
	if _, ok := spec.IsDestroy(ctx); ok {
		return sde.stop(ctx, scriptFile)
	}
	functionName := model.ActionFlags["function-name"]
	if functionName == "" {
		log.Errorf(ctx, "function-name")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "function-name")
	}
	time := model.ActionFlags["time"]
	if time == "" {
		log.Errorf(ctx, "time")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "time")
	}
	t, err := strconv.Atoi(time)
	if err != nil {
		log.Errorf(ctx, "time %v it must be a positive integer", time)
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "time", time, "ti must be a positive integer")
	}
	return sde.start(ctx, scriptFile, functionName, t)
}

func (sde *ScriptDelayExecutor) start(ctx context.Context, scriptFile, functionName string, timt int) *spec.Response {
	timeInSecond := float32(timt) / 1000.0
	// backup file
	response := backScript(ctx, sde.channel, scriptFile)
	if !response.Success {
		return response
	}
	response = insertContentToScriptBy(ctx, sde.channel, functionName, fmt.Sprintf("sleep %f", timeInSecond), scriptFile)
	if !response.Success {
		sde.stop(ctx, scriptFile)
	}
	return response
}

func (sde *ScriptDelayExecutor) stop(ctx context.Context, scriptFile string) *spec.Response {
	return recoverScript(ctx, sde.channel, scriptFile)
}

func (sde *ScriptDelayExecutor) SetChannel(channel spec.Channel) {
	sde.channel = channel
}
