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

package stopprocess

import (
	"context"
	"fmt"
	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/model"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

// init registry provider to model.
func init() {
	model.Provide(new(StopProcess))
}

type StopProcess struct {
	StopProcessName       string `name:"process" json:"process" yaml:"process" default:"" help:"process name"`
	StopProcessInCmd      string `name:"process-cmd" json:"process-cmd" yaml:"process-cmd" default:"" help:"process in command"`
	StartFakeDeath        bool   `name:"start" json:"start" yaml:"start" default:"false" help:"start process fake death"`
	StopFakeDeath         bool   `name:"stop" json:"stop" yaml:"stop" default:"false" help:"recover process fake death"`
	IgnoreProcessNotFound bool   `name:"ignore-not-found" json:"ignore-not-found" yaml:"ignore-not-found" default:"false" help:"ignore process that can't be found"`
	// default arguments
	Channel channel.OsChannel `kong:"-"`
	// for test mock
}

func (that *StopProcess) Assign() model.Worker {
	return &StopProcess{Channel: channel.NewLocalChannel()}
}

func (that *StopProcess) Name() string {
	return exec.StopProcessBin
}

func (that *StopProcess) Exec() *spec.Response {
	if that.StartFakeDeath == that.StopFakeDeath {
		bin.PrintErrAndExit("must add --start or --stop flag")
	}

	if that.StartFakeDeath {
		that.doStopProcess(that.StopProcessName, that.StopProcessInCmd, that.IgnoreProcessNotFound)
	} else if that.StopFakeDeath {
		that.doRecoverProcess(that.StopProcessName, that.StopProcessInCmd, that.IgnoreProcessNotFound)
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
	return spec.ReturnSuccess("")
}

func (that *StopProcess) doStopProcess(process, processCmd string, ignoreProcessNotFound bool) {
	var pids []string
	var err error
	var ctx = context.WithValue(context.Background(), channel.ExcludeProcessKey, "blade")
	if process != "" {
		pids, err = that.Channel.GetPidsByProcessName(process, ctx)
		if err != nil {
			bin.PrintErrAndExit(err.Error())
		}
		that.StopProcessName = process
	} else if processCmd != "" {
		pids, err = that.Channel.GetPidsByProcessCmdName(processCmd, ctx)
		if err != nil {
			bin.PrintErrAndExit(err.Error())
		}
		that.StopProcessName = processCmd
	}
	if pids == nil || len(pids) == 0 {
		if ignoreProcessNotFound {
			bin.PrintOutputAndExit("process not found")
			return
		}
		bin.PrintErrAndExit(fmt.Sprintf("%s process not found", that.StopProcessName))
	}
	args := fmt.Sprintf("-STOP %s", strings.Join(pids, " "))
	response := channel.NewLocalChannel().Run(ctx, "kill", args)
	if !response.Success {
		bin.PrintErrAndExit(response.Err)
	}
	bin.PrintOutputAndExit(response.Result.(string))
}

func (that *StopProcess) doRecoverProcess(process, processCmd string, ignoreProcessNotFound bool) {
	var pids []string
	var err error
	var ctx = context.WithValue(context.Background(), channel.ExcludeProcessKey, "blade")
	if process != "" {
		pids, err = that.Channel.GetPidsByProcessName(process, ctx)
		if err != nil {
			bin.PrintErrAndExit(err.Error())
		}
		that.StopProcessName = process
	} else if processCmd != "" {
		pids, err = that.Channel.GetPidsByProcessCmdName(processCmd, ctx)
		if err != nil {
			bin.PrintErrAndExit(err.Error())
		}
		that.StopProcessName = processCmd
	}

	if pids == nil || len(pids) == 0 {
		if ignoreProcessNotFound {
			bin.PrintOutputAndExit("process not found")
			return
		}
		bin.PrintErrAndExit(fmt.Sprintf("%s process not found", that.StopProcessName))
	}
	response := channel.NewLocalChannel().Run(ctx, "kill", fmt.Sprintf("-CONT %s", strings.Join(pids, " ")))
	if !response.Success {
		bin.PrintErrAndExit(response.Err)
	}
	bin.PrintOutputAndExit(response.Result.(string))
}
