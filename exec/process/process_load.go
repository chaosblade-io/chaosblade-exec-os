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
	osExec "os/exec"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
)

const ProcessLoadBin = "chaos_processload"

type ProcessLoadActionCommandSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewProcessLoadActionCommandSpec() spec.ExpActionCommandSpec {
	return &ProcessLoadActionCommandSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name: "count",
					Desc: "process count, must be a positive integer, 0 or not set means unlimited",
				},
				&spec.ExpFlag{
					Name: "user",
					Desc: "execute process cmd as the specified user",
				},
			},
			ActionFlags:    []spec.ExpFlagSpec{},
			ActionExecutor: &ProcessLoadExecutor{},
			ActionExample: `
# create 10 process as user test
blade create process load --count 10 --user test`,
			ActionPrograms:    []string{ProcessLoadBin},
			ActionCategories:  []string{category.SystemProcess},
			ActionProcessHang: true,
		},
	}
}

func (*ProcessLoadActionCommandSpec) Name() string {
	return "load"
}

func (*ProcessLoadActionCommandSpec) Aliases() []string {
	return []string{"l"}
}

func (*ProcessLoadActionCommandSpec) ShortDesc() string {
	return "create process load"
}

func (l *ProcessLoadActionCommandSpec) LongDesc() string {
	if l.ActionLongDesc != "" {
		return l.ActionLongDesc
	}
	return "create process load by ping"
}

func (*ProcessLoadActionCommandSpec) Categories() []string {
	return []string{category.SystemProcess}
}

type ProcessLoadExecutor struct {
	channel spec.Channel
}

func (pl *ProcessLoadExecutor) Name() string {
	return "load"
}

var localChannel = channel.NewLocalChannel()

func (pl *ProcessLoadExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	countStr := model.ActionFlags["count"]
	userName := model.ActionFlags["user"]

	commands := []string{"ping", "ulimit"}
	if response, ok := pl.channel.IsAllCommandsAvailable(ctx, commands); !ok {
		return response
	}

	if userName != "" {
		_, err := user.Lookup(userName)
		if err != nil {
			log.Errorf(ctx, "could not find user: %v %v\n", userName, err)
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "user", userName, "is invalid")
		}
	}

	if _, ok := spec.IsDestroy(ctx); ok {
		return pl.stop(ctx, userName)
	}

	response := localChannel.Run(ctx, "ulimit", "-u")
	if !response.Success {
		log.Errorf(ctx, "process load, run ulimit err: %s", response.Err)
		return spec.ResponseFailWithFlags(spec.ActionNotSupport, "command ulimit is not support")
	}

	reStr := strings.TrimSpace(response.Result.(string))
	if reStr == "unlimited" {
		log.Errorf(ctx, "proc is unlimited!")
		return spec.ResponseFailWithFlags(spec.ActionNotSupport, "proc is unlimited!")
	}

	_, err := strconv.Atoi(reStr)
	if err != nil {
		return spec.ResponseFailWithFlags(spec.ActionNotSupport, err, "proc is invalid!")
	}

	if countStr == "" {
		countStr = "0"
	}
	count, err := strconv.Atoi(countStr)
	if err != nil {
		log.Errorf(ctx, "count is not a number")
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "count", count, "is not a number")
	}
	if count < 0 {
		log.Errorf(ctx, "count < 0, count is not a illegal parameter")
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "count", count, "must be a positive integer")
	}

	return pl.start(ctx, count, userName)
}

func (pl *ProcessLoadExecutor) SetChannel(channel spec.Channel) {
	pl.channel = channel
}

func (pl *ProcessLoadExecutor) start(ctx context.Context, count int, userName string) *spec.Response {
	if count == 0 {
		if userName != "" {
			go loopProc(ctx, fmt.Sprintf("su -c 'ping 127.0.0.1 > /dev/null' %s", userName))
		} else {
			go loopProc(ctx, fmt.Sprintf("ping 127.0.0.1 > /dev/null"))
		}
		select {}
	} else {
		for i := 0; i < count; i++ {
			if userName != "" {
				simpleProc(ctx, fmt.Sprintf("su -c 'ping 127.0.0.1 > /dev/null' %s", userName))
			} else {
				simpleProc(ctx, fmt.Sprintf("ping 127.0.0.1 > /dev/null"))
			}
		}
	}
	return spec.ReturnSuccess(ctx.Value(spec.Uid))
}

func (pl *ProcessLoadExecutor) stop(ctx context.Context, userName string) *spec.Response {
	if userName != "" {
		simpleProc(ctx, fmt.Sprintf("su -c \"ps aux | grep 'ping 127' | grep -v grep | awk '{print $2}' | xargs kill -9\" %s", userName))
	}
	simpleProc(ctx, "ps aux | grep 'ping 127' | grep -v grep | awk '{print $2}' | xargs kill -9")
	ctx = context.WithValue(ctx, "bin", ProcessLoadBin)
	return exec.Destroy(ctx, pl.channel, "process load")
}

func simpleProc(ctx context.Context, cmd string) {
	if err := osExec.Command("/bin/bash", "-c", cmd).Start(); err != nil {
		log.Errorf(ctx, "exec command: %s failed: %s", cmd, err)
	}
}

func loopProc(ctx context.Context, cmd string) {
	for {
		if err := osExec.Command("/bin/bash", "-c", cmd).Start(); err != nil {
			log.Errorf(ctx, "exec command: %s failed: %s", cmd, err)
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}
