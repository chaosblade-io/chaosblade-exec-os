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
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
	"github.com/sirupsen/logrus"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

var occupiedPort string
var occupiedStart, occupiedStop, occupiedNohup bool

const DefaultPort = ""

func main() {
	flag.StringVar(&occupiedPort, "port", "", "the port occupied")
	flag.BoolVar(&occupiedStart, "start", false, "start operation")
	flag.BoolVar(&occupiedStop, "stop", false, "stop operation")
	flag.BoolVar(&occupiedNohup, "nohup", false, "nohup operation")
	bin.ParseFlagAndInitLog()

	if occupiedPort == DefaultPort {
		bin.PrintAndExitWithErrPrefix("illegal port value")
	}
	if occupiedStart {
		startOccupy(occupiedPort)
	} else if occupiedStop {
		stopOccupy(occupiedPort)
	} else {
		bin.PrintAndExitWithErrPrefix("less --start or --stop flag")
	}

}

var occupyLogFile = util.GetNohupOutput(util.Bin, "chaos_occupynetwork.log")

func startOccupy(port string) {
	if occupiedNohup {
		err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
		if err != nil {
			bin.PrintAndExitWithErrPrefix(err.Error())
		}
	} else {
		// start the program
		channel := channel.NewLocalChannel()
		ctx := context.Background()
		response := channel.Run(ctx, "nohup",
			fmt.Sprintf(`%s --start --port %s --nohup=true > %s 2>&1 &`,
				path.Join(util.GetProgramPath(), exec.OccupyNetworkBin), port, occupyLogFile))
		if !response.Success {
			bin.PrintErrAndExit(response.Err)
		}
		// check
		time.Sleep(time.Second)
		response = channel.Run(ctx, "grep", fmt.Sprintf("%s %s", bin.ErrPrefix, occupyLogFile))
		if response.Success {
			errMsg := strings.TrimSpace(response.Result.(string))
			if errMsg != "" {
				bin.PrintErrAndExit(errMsg)
			}
		}
	}
}

func stopOccupy(port string) {
	chl := channel.NewLocalChannel()
	ctx := context.WithValue(context.Background(), channel.ProcessKey, exec.OccupyNetworkBin)
	pids, err := chl.GetPidsByProcessName(fmt.Sprintf(`port %s --nohup`, port), ctx)
	if err != nil {
		logrus.Warnf("get %s pid failed, %v", exec.OccupyNetworkBin, err)
	}
	if pids != nil || len(pids) >= 0 {
		chl.Run(ctx, "kill", fmt.Sprintf("-9 %s", strings.Join(pids, " ")))
	}
	chl.Run(ctx, "rm", fmt.Sprintf("-rf %s*", occupyLogFile))
}
