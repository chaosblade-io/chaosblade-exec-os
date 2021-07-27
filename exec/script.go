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
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
)

type ScriptCommandModelSpec struct {
	spec.BaseExpModelCommandSpec
}

func NewScriptCommandModelSpec() spec.ExpModelCommandSpec {
	return &ScriptCommandModelSpec{
		spec.BaseExpModelCommandSpec{
			ExpFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:                  "file",
					Desc:                  "Script file full path",
					Required:              true,
					RequiredWhenDestroyed: true,
				},
				&spec.ExpFlag{
					Name:     "function-name",
					Desc:     "function name in shell",
					Required: true,
				},
			},
			ExpActions: []spec.ExpActionCommandSpec{
				NewScriptDelayActionCommand(),
				NewScriptExitActionCommand(),
			},
		},
	}
}

func (*ScriptCommandModelSpec) Name() string {
	return "script"
}

func (*ScriptCommandModelSpec) ShortDesc() string {
	return "Script chaos experiment"
}

func (*ScriptCommandModelSpec) LongDesc() string {
	return "Script chaos experiment"
}

const bakFileSuffix = "_chaosblade.bak"

// backScript
func backScript(channel spec.Channel, scriptFile string) *spec.Response {
	var bakFile = getBackFile(scriptFile)
	if util.IsExist(bakFile) {
		return spec.ResponseFailWithFlags(spec.BackfileExists, bakFile)
	}
	return channel.Run(context.TODO(), "cat", fmt.Sprintf("%s > %s", scriptFile, bakFile))
}

func recoverScript(channel spec.Channel, scriptFile string) *spec.Response {
	var bakFile = getBackFile(scriptFile)
	if !util.IsExist(bakFile) {
		return spec.ResponseFailWithFlags(spec.FileNotExist, bakFile)
	}
	response := channel.Run(context.TODO(), "cat", fmt.Sprintf("%s > %s", bakFile, scriptFile))
	if !response.Success {
		return response
	}
	return channel.Run(context.TODO(), "rm", fmt.Sprintf("-rf %s", bakFile))
}

func getBackFile(scriptFile string) string {
	return scriptFile + bakFileSuffix
}

// awk '/offline\s?\(\)\s*\{/{print NR}' tt.sh
// sed -i '416 a sleep 100' tt.sh
func insertContentToScriptBy(channel spec.Channel, functionName string, newContent, scriptFile string) *spec.Response {
	// search line number by function name
	response := channel.Run(context.TODO(), "awk", fmt.Sprintf(`'/%s *\(\) *\{/{print NR}' %s`, functionName, scriptFile))
	if !response.Success {
		return response
	}
	result := strings.TrimSpace(response.Result.(string))
	lineNums := strings.Split(result, "\n")
	if len(lineNums) > 1 {
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "function-name", functionName,
			"the function name must be unique in the script")
	}
	if len(lineNums) == 0 || strings.TrimSpace(lineNums[0]) == "" {
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "function-name", functionName,
			"cannot find the function name in the script")
	}
	lineNum := lineNums[0]
	// insert content to the line below
	return channel.Run(context.TODO(), "sed", fmt.Sprintf(`-i '%s a %s' %s`, lineNum, newContent, scriptFile))
}
