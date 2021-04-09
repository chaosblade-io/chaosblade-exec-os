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

package occupynetwork

import (
	"context"
	"fmt"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/model"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
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

// init registry provider to model.
func init() {
	model.Provide(new(OccupyNetwork))
}

type OccupyNetwork struct {
	OccupiedPort  string `name:"port" json:"port" yaml:"port" default:"" help:"the port occupied"`
	OccupiedStart bool   `name:"start" json:"start" yaml:"start" default:"false" help:"start operation"`
	OccupiedStop  bool   `name:"stop" json:"stop" yaml:"stop" default:"false" help:"stop operation"`
	OccupiedNohup bool   `name:"nohup" json:"nohup" yaml:"nohup" default:"false" help:"nohup operation"`
	// default arguments
	Channel channel.OsChannel `kong:"-"`
	// for test mock
}

func (that *OccupyNetwork) Assign() model.Worker {
	return &OccupyNetwork{Channel: channel.NewLocalChannel()}
}

func (that *OccupyNetwork) Name() string {
	return exec.OccupyNetworkBin
}

func (that *OccupyNetwork) Exec() *spec.Response {
	if that.OccupiedPort == DefaultPort {
		bin.PrintAndExitWithErrPrefix("illegal port value")
	}
	if that.OccupiedStart {
		that.startOccupy(that.OccupiedPort)
	} else if that.OccupiedStop {
		that.stopOccupy(that.OccupiedPort)
	} else {
		bin.PrintAndExitWithErrPrefix("less --start or --stop flag")
	}
	return spec.ReturnSuccess("")
}

const DefaultPort = ""

var occupyLogFile = util.GetNohupOutput(util.Bin, "chaos_occupynetwork.log")

func (that *OccupyNetwork) startOccupy(port string) {
	if that.OccupiedNohup {
		err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
		if err != nil {
			bin.PrintAndExitWithErrPrefix(err.Error())
		}
	} else {
		// start the program
		ctx := context.Background()
		response := that.Channel.Run(ctx, "nohup",
			fmt.Sprintf(`%s --start --port %s --nohup=true > %s 2>&1 &`,
				path.Join(util.GetProgramPath(), exec.OccupyNetworkBin), port, occupyLogFile))
		if !response.Success {
			bin.PrintErrAndExit(response.Err)
		}
		// check
		time.Sleep(time.Second)
		response = that.Channel.Run(ctx, "grep", fmt.Sprintf("%s %s", bin.ErrPrefix, occupyLogFile))
		if response.Success {
			errMsg := strings.TrimSpace(response.Result.(string))
			if errMsg != "" {
				bin.PrintErrAndExit(errMsg)
			}
		}
	}
}

func (that *OccupyNetwork) stopOccupy(port string) {
	ctx := context.WithValue(context.Background(), channel.ProcessKey, exec.OccupyNetworkBin)
	pids, err := that.Channel.GetPidsByProcessName(fmt.Sprintf(`port %s --nohup`, port), ctx)
	if err != nil {
		logrus.Warnf("get %s pid failed, %v", exec.OccupyNetworkBin, err)
	}
	if pids != nil || len(pids) >= 0 {
		that.Channel.Run(ctx, "kill", fmt.Sprintf("-9 %s", strings.Join(pids, " ")))
	}
	that.Channel.Run(ctx, "rm", fmt.Sprintf("-rf %s*", occupyLogFile))
}
