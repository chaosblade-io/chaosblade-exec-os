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

	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
)

type ScriptExitActionCommand struct {
	spec.BaseExpActionCommandSpec
}

func NewScriptExitActionCommand() spec.ExpActionCommandSpec {
	return &ScriptExitActionCommand{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{},
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "exit-code",
					Desc:     "Exit code",
					Required: false,
				},
				&spec.ExpFlag{
					Name:     "exit-message",
					Desc:     "Exit message",
					Required: false,
				},
			},
			ActionExecutor: &ScriptExitExecutor{},
		},
	}
}

func (*ScriptExitActionCommand) Name() string {
	return "exit"
}

func (*ScriptExitActionCommand) Aliases() []string {
	return []string{}
}

func (*ScriptExitActionCommand) ShortDesc() string {
	return "Exit script"
}

func (*ScriptExitActionCommand) LongDesc() string {
	return "Exit script with specify message and code"
}

type ScriptExitExecutor struct {
	channel spec.Channel
}

func (*ScriptExitExecutor) Name() string {
	return "exit"
}

func (see *ScriptExitExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	err := checkScriptExpEnv()
	if err != nil {
		return spec.ReturnFail(spec.Code[spec.CommandNotFound], err.Error())
	}
	if see.channel == nil {
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
		return see.stop(ctx, scriptFile)
	}
	functionName := model.ActionFlags["function-name"]
	if functionName == "" {
		return spec.ReturnFail(spec.Code[spec.IllegalParameters], "must specify --function-name flag")
	}
	exitMessage := model.ActionFlags["exit-message"]
	exitCode := model.ActionFlags["exit-code"]
	return see.start(ctx, scriptFile, functionName, exitMessage, exitCode)
}

func (see *ScriptExitExecutor) start(ctx context.Context, scriptFile, functionName, exitMessage, exitCode string) *spec.Response {
	var content string
	if exitMessage != "" {
		content = fmt.Sprintf(`echo "%s";`, exitMessage)
	}
	if exitCode == "" {
		exitCode = "1"
	}
	content = fmt.Sprintf("%sexit %s", content, exitCode)
	// backup file
	response := backScript(see.channel, scriptFile)
	if !response.Success {
		return response
	}
	response = insertContentToScriptBy(see.channel, functionName, content, scriptFile)
	if !response.Success {
		see.stop(ctx, scriptFile)
	}
	return response
}

func (see *ScriptExitExecutor) stop(ctx context.Context, scriptFile string) *spec.Response {
	return recoverScript(see.channel, scriptFile)
}

func (see *ScriptExitExecutor) SetChannel(channel spec.Channel) {
	see.channel = channel
}
