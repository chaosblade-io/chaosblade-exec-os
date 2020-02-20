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

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

var stopProcessName string
var stopProcessInCmd string

var startFakeDeath, stopFakeDeath bool

func main() {
	flag.StringVar(&stopProcessName, "process", "", "process name")
	flag.StringVar(&stopProcessInCmd, "process-cmd", "", "process in command")
	flag.BoolVar(&startFakeDeath, "start", false, "start process fake death")
	flag.BoolVar(&stopFakeDeath, "stop", false, "recover process fake death")
	bin.ParseFlagAndInitLog()

	if startFakeDeath == stopFakeDeath {
		bin.PrintErrAndExit("must add --start or --stop flag")
	}

	if startFakeDeath {
		doStopProcess(stopProcessName, stopProcessInCmd)
	} else if stopFakeDeath {
		doRecoverProcess(stopProcessName, stopProcessInCmd)
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
}

var cl = channel.NewLocalChannel()

func doStopProcess(process, processCmd string) {
	var pids []string
	var err error
	var ctx = context.WithValue(context.Background(), channel.ExcludeProcessKey, "blade")
	if process != "" {
		pids, err = cl.GetPidsByProcessName(process, ctx)
		if err != nil {
			bin.PrintErrAndExit(err.Error())
		}
		stopProcessName = process
	} else if processCmd != "" {
		pids, err = cl.GetPidsByProcessCmdName(processCmd, ctx)
		if err != nil {
			bin.PrintErrAndExit(err.Error())
		}
		stopProcessName = processCmd
	}

	if pids == nil || len(pids) == 0 {
		bin.PrintErrAndExit(fmt.Sprintf("%s process not found", stopProcessName))
	}
	args := fmt.Sprintf("-STOP %s", strings.Join(pids, " "))
	response := channel.NewLocalChannel().Run(ctx, "kill", args)
	if !response.Success {
		bin.PrintErrAndExit(response.Err)
	}
	bin.PrintOutputAndExit(response.Result.(string))
}

func doRecoverProcess(process, processCmd string) {
	var pids []string
	var err error
	var ctx = context.WithValue(context.Background(), channel.ExcludeProcessKey, "blade")
	if process != "" {
		pids, err = cl.GetPidsByProcessName(process, ctx)
		if err != nil {
			bin.PrintErrAndExit(err.Error())
		}
		stopProcessName = process
	} else if processCmd != "" {
		pids, err = cl.GetPidsByProcessCmdName(processCmd, ctx)
		if err != nil {
			bin.PrintErrAndExit(err.Error())
		}
		stopProcessName = processCmd
	}

	if pids == nil || len(pids) == 0 {
		bin.PrintErrAndExit(fmt.Sprintf("%s process not found", stopProcessName))
	}
	response := channel.NewLocalChannel().Run(ctx, "kill", fmt.Sprintf("-CONT %s", strings.Join(pids, " ")))
	if !response.Success {
		bin.PrintErrAndExit(response.Err)
	}
	bin.PrintOutputAndExit(response.Result.(string))
}
