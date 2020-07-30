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

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
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

func (*ScriptCommandModelSpec) Example() string {
	return `blade create script delay --time 2000 --file xxx.sh --function-name start

blade create script exit --file xxx.sh --function-name offline --exit-message "error" --exit-code 2`
}

const bakFileSuffix = "_chaosblade.bak"

// backScript
func backScript(channel spec.Channel, scriptFile string) *spec.Response {
	var bakFile = getBackFile(scriptFile)
	if util.IsExist(bakFile) {
		return spec.ReturnFail(spec.Code[spec.StatusError],
			fmt.Sprintf("%s backup file exists, may be annother experiment is running", bakFile))
	}
	return channel.Run(context.TODO(), "cat", fmt.Sprintf("%s > %s", scriptFile, bakFile))
}

func recoverScript(channel spec.Channel, scriptFile string) *spec.Response {
	var bakFile = getBackFile(scriptFile)
	if !util.IsExist(bakFile) {
		return spec.ReturnFail(spec.Code[spec.FileNotFound],
			fmt.Sprintf("%s backup file not exists", bakFile))
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
		return spec.ReturnFail(spec.Code[spec.IllegalParameters],
			fmt.Sprintf("get too many lines by the %s function name", functionName))
	}
	if len(lineNums) == 0 || strings.TrimSpace(lineNums[0]) == "" {
		return spec.ReturnFail(spec.Code[spec.IllegalParameters],
			fmt.Sprintf("cannot find the %s function name", functionName))
	}
	lineNum := lineNums[0]
	// insert content to the line below
	return channel.Run(context.TODO(), "sed", fmt.Sprintf(`-i '%s a %s' %s`, lineNum, newContent, scriptFile))
}

func checkScriptExpEnv() error {
	commands := []string{"cat", "rm", "sed", "awk", "rm"}
	for _, command := range commands {
		if !channel.NewLocalChannel().IsCommandAvailable(command) {
			return fmt.Errorf("%s command not found", command)
		}
	}
	return nil
}
