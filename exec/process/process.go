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
	"fmt"
	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
	"strconv"
	"strings"
)

type ProcessCommandModelSpec struct {
	spec.BaseExpModelCommandSpec
}

func NewProcessCommandModelSpec() spec.ExpModelCommandSpec {
	return &ProcessCommandModelSpec{
		spec.BaseExpModelCommandSpec{
			ExpFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:   "ignore-not-found",
					Desc:   "Ignore process that cannot be found",
					NoArgs: true,
				},
			},
			ExpActions: []spec.ExpActionCommandSpec{
				NewKillProcessActionCommandSpec(),
				NewStopProcessActionCommandSpec(),
			},
		},
	}
}

func (*ProcessCommandModelSpec) Name() string {
	return "process"
}

func (*ProcessCommandModelSpec) ShortDesc() string {
	return "Process experiment"
}

func (*ProcessCommandModelSpec) LongDesc() string {
	return "Process experiment, for example, kill process"
}

func getPids(ctx context.Context, cl spec.Channel, model *spec.ExpModel, uid string) *spec.Response {

	countValue := model.ActionFlags["count"]
	process := model.ActionFlags["process"]
	processCmd := model.ActionFlags["process-cmd"]
	localPorts := model.ActionFlags["local-port"]
	pid := model.ActionFlags["pid"]

	excludeProcess := model.ActionFlags["exclude-process"]
	ignoreProcessNotFound := model.ActionFlags["ignore-not-found"] == "true"
	if process == "" && processCmd == "" && localPorts == "" && pid == "" {
		log.Errorf(ctx, "pid、less process、process-cmd and local-port, less process matcher")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "pid|process|process-cmd|local-port")
	}

	var excludeProcessValue = fmt.Sprintf("blade,%s", excludeProcess)
	ctx = context.WithValue(ctx, channel.ExcludeProcessKey, excludeProcessValue)
	if !ignoreProcessNotFound {
		if response := checkProcessInvalid(ctx, process, processCmd, localPorts, pid, cl); response != nil {
			return response
		}
	}
	flags := fmt.Sprintf("--debug=%t", util.Debug)
	if countValue != "" {
		count, err := strconv.Atoi(countValue)
		if err != nil {
			log.Errorf(ctx, spec.ParameterIllegal.Sprintf("count", countValue, err))
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "count", countValue, err)
		}
		flags = fmt.Sprintf("%s --count %d", flags, count)
	}
	if process != "" {
		flags = fmt.Sprintf(`%s --process "%s"`, flags, process)
	} else if processCmd != "" {
		flags = fmt.Sprintf(`%s --process-cmd "%s"`, flags, processCmd)
	} else if localPorts != "" {
		flags = fmt.Sprintf(`%s --local-port "%s"`, flags, localPorts)
	}

	if excludeProcess != "" {
		flags = fmt.Sprintf(`%s --exclude-process %s`, flags, excludeProcess)
	}
	if ignoreProcessNotFound {
		flags = fmt.Sprintf(`%s --ignore-not-found=%t`, flags, ignoreProcessNotFound)
	}

	var pids []string
	var err error
	var killProcessName string
	ctx = context.WithValue(ctx, channel.ExcludeProcessKey, excludeProcessValue)
	if process != "" {
		pids, err = cl.GetPidsByProcessName(process, ctx)
		if err != nil {
			return spec.ReturnFail(spec.OsCmdExecFailed, fmt.Sprintf("get pids by processname err, %v", err))
		}
		killProcessName = process
	} else if processCmd != "" {
		pids, err = cl.GetPidsByProcessCmdName(processCmd, ctx)
		if err != nil {
			return spec.ReturnFail(spec.OsCmdExecFailed, fmt.Sprintf("get pids by processcmdname err, %v", err))
		}
		killProcessName = processCmd
	} else if localPorts != "" {
		ports, err := util.ParseIntegerListToStringSlice("local-port", localPorts)
		if err != nil {
			return spec.ReturnFail(spec.ParameterIllegal, fmt.Sprintf("illegal parameter local-port, %v", err))
		}
		pids, err = cl.GetPidsByLocalPorts(ctx, ports)
		if err != nil {
			return spec.ReturnFail(spec.ParameterIllegal, fmt.Sprintf("illegal parameter ports, %v", err))
		}
	} else if pid != "" {
		tempPidList := strings.Split(pid, ",")
		pids = append(pids, tempPidList...)
	}
	if pids == nil || len(pids) == 0 {
		if ignoreProcessNotFound {
			return spec.Success()
		}
		return spec.ReturnFail(spec.OsCmdExecFailed, fmt.Sprintf("%s process not found", killProcessName))
	}
	count, _ := strconv.Atoi(countValue)
	// remove duplicates
	pids = util.RemoveDuplicates(pids)
	if count > 0 && len(pids) > count {
		pids = pids[:count]
	}
	return spec.ReturnSuccess(strings.Join(pids, " "))
}

func checkProcessInvalid(ctx context.Context, process, processCmd, localPorts, pid string, cl spec.Channel) *spec.Response {
	var pids []string
	var killProcessName string
	var err error
	var processParameter string
	if process != "" {
		pids, err = cl.GetPidsByProcessName(process, ctx)
		if err != nil {
			log.Errorf(ctx, spec.ProcessIdByNameFailed.Sprintf(process, err))
			return spec.ResponseFailWithFlags(spec.ProcessIdByNameFailed, process, err)
		}
		killProcessName = process
		processParameter = "process"
	} else if processCmd != "" {
		pids, err = cl.GetPidsByProcessCmdName(processCmd, ctx)
		if err != nil {
			log.Errorf(ctx, spec.ProcessIdByNameFailed.Sprintf(processCmd, err))
			return spec.ResponseFailWithFlags(spec.ProcessIdByNameFailed, processCmd, err)
		}
		killProcessName = processCmd
		processParameter = "process-cmd"
	} else if localPorts != "" {
		ports, err := util.ParseIntegerListToStringSlice("local-port", localPorts)
		if err != nil {
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "local-port", localPorts, err)
		}
		pids, err = cl.GetPidsByLocalPorts(ctx, ports)
		killProcessName = localPorts
		processParameter = "local-port"
	} else if pid != "" {
		pids = append(pids, pid)
		killProcessName = pid
		processParameter = "pid"
	}
	if pids == nil || len(pids) == 0 {
		return spec.ResponseFailWithFlags(spec.ParameterInvalidProName, processParameter, killProcessName)
	}
	return nil
}
