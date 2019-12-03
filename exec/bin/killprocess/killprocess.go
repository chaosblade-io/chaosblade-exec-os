/*
 * Copyright 1999-2019 Alibaba Group Holding Ltd.
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

var killProcessName string
var killProcessInCmd string
var killProcessCount int

func main() {
	flag.StringVar(&killProcessName, "process", "", "process name")
	flag.StringVar(&killProcessInCmd, "process-cmd", "", "process in command")
	flag.IntVar(&killProcessCount, "count", 0, "limit count")
	bin.ParseFlagAndInitLog()

	killProcess(killProcessName, killProcessInCmd, killProcessCount)
}

func killProcess(process, processCmd string, count int) {
	var pids []string
	var err error
	var ctx = context.WithValue(context.Background(), channel.ExcludeProcessKey, "blade")
	if process != "" {
		pids, err = channel.GetPidsByProcessName(process, ctx)
		if err != nil {
			bin.PrintErrAndExit(err.Error())
		}
		killProcessName = process
	} else if processCmd != "" {
		pids, err = channel.GetPidsByProcessCmdName(processCmd, ctx)
		if err != nil {
			bin.PrintErrAndExit(err.Error())
		}
		killProcessName = processCmd
	}

	if pids == nil || len(pids) == 0 {
		bin.PrintErrAndExit(fmt.Sprintf("%s process not found", killProcessName))
		return
	}
	if count > 0 && len(pids) > count {
		pids = pids[:count]
	}
	response := channel.NewLocalChannel().Run(ctx, "kill", fmt.Sprintf("-9 %s", strings.Join(pids, " ")))
	if !response.Success {
		bin.PrintErrAndExit(response.Err)
		return
	}
	bin.PrintOutputAndExit(response.Result.(string))
}
