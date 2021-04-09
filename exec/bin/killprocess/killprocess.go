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

package killprocess

import (
	"context"
	"fmt"
	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/model"
	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
	"strings"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

// init registry provider to model.
func init() {
	model.Provide(new(KillProcess))
}

type KillProcess struct {
	KillProcessName           string `name:"process" json:"process" yaml:"process" default:"" help:"process name"`
	KillProcessInCmd          string `name:"process-cmd" json:"process-cmd" yaml:"process-cmd" default:"" help:"process in command"`
	KillProcessCount          int    `name:"count" json:"count" yaml:"count" default:"0" help:"limit count"`
	KillProcessLocalPorts     string `name:"local-port" json:"local-port" yaml:"local-port" default:"" help:"local service ports"`
	KillProcessSignal         string `name:"signal" json:"signal" yaml:"signal" default:"9" help:"kill process signal"`
	KillProcessExcludeProcess string `name:"exclude-process" json:"exclude-process" yaml:"exclude-process" default:"" help:"kill process exclude specific process"`
	IgnoreProcessNotFound     bool   `name:"ignore-not-found" json:"ignore-not-found" yaml:"ignore-not-found" default:"false" help:"ignore process that can't be found"`
	// default arguments
	Channel channel.OsChannel `kong:"-"`
	// for test mock
}

func (that *KillProcess) Assign() model.Worker {
	return &KillProcess{Channel: channel.NewLocalChannel()}
}

func (that *KillProcess) Name() string {
	return exec.KillProcessBin
}

func (that *KillProcess) Exec() *spec.Response {
	that.killProcess(
		that.KillProcessName,
		that.KillProcessInCmd,
		that.KillProcessLocalPorts,
		that.KillProcessSignal,
		that.KillProcessExcludeProcess,
		that.KillProcessCount,
		that.IgnoreProcessNotFound)
	return spec.ReturnSuccess("")
}

func (that *KillProcess) killProcess(process, processCmd, localPorts, signal, excludeProcess string, count int, ignoreProcessNotFound bool) {
	var pids []string
	var err error
	var excludeProcessValue = fmt.Sprintf("blade,%s", excludeProcess)
	var ctx = context.WithValue(context.Background(), channel.ExcludeProcessKey, excludeProcessValue)
	if process != "" {
		pids, err = that.Channel.GetPidsByProcessName(process, ctx)
		if err != nil {
			bin.PrintErrAndExit(err.Error())
		}
		that.KillProcessName = process
	} else if processCmd != "" {
		pids, err = that.Channel.GetPidsByProcessCmdName(processCmd, ctx)
		if err != nil {
			bin.PrintErrAndExit(err.Error())
		}
		that.KillProcessName = processCmd
	} else if localPorts != "" {
		ports, err := util.ParseIntegerListToStringSlice(localPorts)
		if err != nil {
			bin.PrintErrAndExit(err.Error())
		}
		pids, err = that.Channel.GetPidsByLocalPorts(ports)
		if err != nil {
			bin.PrintErrAndExit(err.Error())
		}
	}
	if pids == nil || len(pids) == 0 {
		if ignoreProcessNotFound {
			bin.PrintOutputAndExit("process not found")
			return
		}
		bin.PrintErrAndExit(fmt.Sprintf("%s process not found", that.KillProcessName))
		return
	}
	// remove duplicates
	pids = util.RemoveDuplicates(pids)
	if count > 0 && len(pids) > count {
		pids = pids[:count]
	}
	response := that.Channel.Run(ctx, "kill", fmt.Sprintf("-%s %s", signal, strings.Join(pids, " ")))
	if !response.Success {
		bin.PrintErrAndExit(response.Err)
		return
	}
	bin.PrintOutputAndExit(response.Result.(string))
}
