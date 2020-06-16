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

package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

var killProcessName, killProcessInCmd, killProcessLocalPorts, killProcessSignal, killProcessExcludeProcess string
var killProcessCount int

func main() {
	flag.StringVar(&killProcessName, "process", "", "process name")
	flag.StringVar(&killProcessInCmd, "process-cmd", "", "process in command")
	flag.IntVar(&killProcessCount, "count", 0, "limit count")
	flag.StringVar(&killProcessLocalPorts, "local-port", "", "local service ports")
	flag.StringVar(&killProcessSignal, "signal", "9", "kill process signal")
	flag.StringVar(&killProcessExcludeProcess, "exclude-process", "", "kill process exclude specific process")
	bin.ParseFlagAndInitLog()

	killProcess(killProcessName, killProcessInCmd, killProcessLocalPorts, killProcessSignal, killProcessExcludeProcess, killProcessCount)
}

var cl = channel.NewLocalChannel()

func killProcess(process, processCmd, localPorts, signal, excludeProcess string, count int) {
	var pids []string
	var err error
	var excludeProcessValue = fmt.Sprintf("blade,%s", excludeProcess)
	var ctx = context.WithValue(context.Background(), channel.ExcludeProcessKey, excludeProcessValue)
	if process != "" {
		pids, err = cl.GetPidsByProcessName(process, ctx)
		if err != nil {
			bin.PrintErrAndExit(err.Error())
		}
		killProcessName = process
	} else if processCmd != "" {
		pids, err = cl.GetPidsByProcessCmdName(processCmd, ctx)
		if err != nil {
			bin.PrintErrAndExit(err.Error())
		}
		killProcessName = processCmd
	} else if localPorts != "" {
		ports, err := util.ParseIntegerListToStringSlice(localPorts)
		if err != nil {
			bin.PrintErrAndExit(err.Error())
		}
		pids, err = cl.GetPidsByLocalPorts(ports)
		if err != nil {
			bin.PrintErrAndExit(err.Error())
		}
	}
	if pids == nil || len(pids) == 0 {
		bin.PrintErrAndExit(fmt.Sprintf("%s process not found", killProcessName))
		return
	}
	// remove duplicates
	pids = util.RemoveDuplicates(pids)
	if count > 0 && len(pids) > count {
		pids = pids[:count]
	}
	response := cl.Run(ctx, "kill", fmt.Sprintf("-%s %s", signal, strings.Join(pids, " ")))
	if !response.Success {
		bin.PrintErrAndExit(response.Err)
		return
	}
	bin.PrintOutputAndExit(response.Result.(string))
}
