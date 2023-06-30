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
	"os"
	"strconv"
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/shirou/gopsutil/process"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
)

const KillProcessBin = "chaos_killprocess"

type KillProcessActionCommandSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewKillProcessActionCommandSpec() spec.ExpActionCommandSpec {
	return &KillProcessActionCommandSpec{
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
					Name: "signal",
					Desc: "Killing process signal, only 9 or 15",
				},
			},
			ActionFlags:    []spec.ExpFlagSpec{},
			ActionExecutor: &KillProcessExecutor{},
			ActionExample: `
# Kill the process that contains the SimpleHTTPServer keyword
blade create process kill --process SimpleHTTPServer --signal 15

# Kill the Java process
blade create process kill --process-cmd java.exe --signal 15`,
			ActionPrograms:   []string{KillProcessBin},
			ActionCategories: []string{category.SystemProcess},
		},
	}
}

func (*KillProcessActionCommandSpec) Name() string {
	return "kill"
}

func (*KillProcessActionCommandSpec) Aliases() []string {
	return []string{"k"}
}

func (*KillProcessActionCommandSpec) ShortDesc() string {
	return "Kill process"
}

func (k *KillProcessActionCommandSpec) LongDesc() string {
	if k.ActionLongDesc != "" {
		return k.ActionLongDesc
	}
	return "Kill process by process id or process name"
}

func (*KillProcessActionCommandSpec) Categories() []string {
	return []string{category.SystemProcess}
}

type KillProcessExecutor struct {
	channel spec.Channel
}

func (kpe *KillProcessExecutor) Name() string {
	return "kill"
}

func (kpe *KillProcessExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	if _, ok := spec.IsDestroy(ctx); ok {
		return spec.ReturnSuccess(uid)
	}

	resp := getPids(ctx, kpe.channel, model, uid)
	if !resp.Success {
		return resp
	}
	pids := resp.Result.(string)
	signal := model.ActionFlags["signal"]

	if signal == "" {
		log.Errorf(ctx, "less signal flag value")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "signal")
	}
	return kpe.KillProcessByPids(ctx, pids, signal)
}

func (kpe *KillProcessExecutor) KillProcessByPids(ctx context.Context, pids, signal string) *spec.Response {
	arrPids := strings.Split(pids, " ")
	for _, pid := range arrPids {
		ipid, err := strconv.ParseInt(pid, 10, 32)
		if err != nil {
			log.Errorf(ctx, "`%s`, get pid failed, err: %s", pids, err.Error())
			return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "Get PID", err.Error())
		}

		switch signal {
		case "9":
			prce, err := os.FindProcess(int(ipid))
			if err != nil {
				log.Errorf(ctx, "`%s`, get pid info failed, err: %s", pids, err.Error())
				return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "Get PID info", err.Error())
			}
			err = prce.Kill()
			if err != nil {
				log.Errorf(ctx, "`%v`, kill process failed, err: %s", pid, err.Error())
				return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "kill process", err.Error())
			}
		case "15":
			prce, err := process.NewProcessWithContext(context.Background(), int32(ipid))
			if err != nil {
				log.Errorf(ctx, "`%s`, get pid info failed, err: %s", pids, err.Error())
				return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "Get PID info", err.Error())
			}
			err = prce.Terminate()
			if err != nil {
				log.Errorf(ctx, "`%v`, terminate process failed, err: %s", pid, err.Error())
				return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "terminate process", err.Error())
			}
		default:
			log.Errorf(ctx, "`%v`, kill process failed, the signal not support", pid)
			return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "kill process", "signal not support")
		}
	}
	return spec.Success()
}

func (kpe *KillProcessExecutor) SetChannel(channel spec.Channel) {
	kpe.channel = channel
}
