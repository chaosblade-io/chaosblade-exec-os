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

package straceerror

import (
	"context"
	"fmt"
	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/model"
	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
)

// init registry provider to model.
func init() {
	model.Provide(new(KernelError))
}

type KernelError struct {
	StraceErrorStart bool   `name:"start" json:"start" yaml:"start" default:"false" help:"start fail syscall"`
	StraceErrorStop  bool   `name:"stop" json:"stop" yaml:"stop" default:"false" help:"stop fail syscall"`
	StraceErrorNohup bool   `name:"nohup" json:"nohup" yaml:"nohup" default:"false" help:"nohup to run fail syscall"`
	PidList          string `name:"pid" json:"pid" yaml:"pid" default:"" help:"pids of affected processes"`
	SyscallName      string `name:"syscall-name" json:"syscall-name" yaml:"syscall-name" default:"" help:"failed syscall"`
	ReturnValue      string `name:"return-value" json:"return-value" yaml:"return-value" default:"" help:"injected return value"`
	First            string `name:"first" json:"first" yaml:"first" default:"" help:"the first failed syscall"`
	End              string `name:"end" json:"end" yaml:"end" default:"" help:"the last failed syscall"`
	Step             string `name:"step" json:"step" yaml:"step" default:"" help:"the interval between failed syscall"`
	// default arguments
	Channel channel.OsChannel `kong:"-"`
	// for test mock
}

func (that *KernelError) Assign() model.Worker {
	return &KernelError{Channel: channel.NewLocalChannel()}
}

func (that *KernelError) Name() string {
	return exec.StraceErrorBin
}

func (that *KernelError) Exec() *spec.Response {
	if that.StraceErrorStart {
		that.startError()
	} else if that.StraceErrorStop {
		if success, errs := that.stopError(); !success {
			bin.PrintErrAndExit(errs)
		}
	} else if that.StraceErrorNohup {
		go that.errorNohup()

		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGHUP, syscall.SIGKILL, syscall.SIGTERM)
		for s := range ch {
			switch s {
			case syscall.SIGHUP, syscall.SIGKILL, syscall.SIGTERM, os.Interrupt:
				fmt.Printf("caught interrupt, exit")
				return spec.ReturnSuccess("")
			}
		}
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
	return spec.ReturnSuccess("")
}

func (that *KernelError) startError() {
	args := fmt.Sprintf("%s --nohup --pid %s --syscall-name %s --return-value %s",
		path.Join(util.GetProgramPath(), that.Name()), that.PidList, that.SyscallName, that.ReturnValue)
	if that.First != "" {
		args = fmt.Sprintf("%s --first %s", args, that.First)
	}
	if that.End != "" {
		args = fmt.Sprintf("%s --end %s", args, that.End)
	}
	if that.Step != "" {
		args = fmt.Sprintf("%s --step %s", args, that.Step)
	}
	args = fmt.Sprintf("%s > /dev/null 2>&1 &", args)
	ctx := context.Background()
	response := that.Channel.Run(ctx, "nohup", args)

	if !response.Success {
		that.stopError()
		bin.PrintErrAndExit(response.Err)
	}
}

func (that *KernelError) stopError() (success bool, errs string) {
	ctx := context.WithValue(context.Background(), channel.ProcessKey, "nohup")
	pids, _ := that.Channel.GetPidsByProcessName(that.Name(), ctx)
	if pids == nil || len(pids) == 0 {
		return true, errs
	}
	response := that.Channel.Run(ctx, "kill", fmt.Sprintf(`-HUP %s`, strings.Join(pids, " ")))
	if !response.Success {
		return false, response.Err
	}
	return true, errs
}

func (that *KernelError) errorNohup() {
	if that.PidList != "" {
		pids := strings.Split(that.PidList, ",")
		args := fmt.Sprintf("-f -e inject=%s:error=%s", that.SyscallName, that.ReturnValue)

		if that.First != "" {
			args = fmt.Sprintf("%s:when=%s", args, that.First)
			if that.Step != "" && that.End != "" {
				args = fmt.Sprintf("%s..%s+%s", args, that.End, that.Step)
			} else if that.Step != "" {
				args = fmt.Sprintf("%s+%s", args, that.Step)
			} else if that.End != "" {
				args = fmt.Sprintf("%s..%s", args, that.End)
			}
		}

		for _, pid := range pids {
			args = fmt.Sprintf("-p %s %s", pid, args)
		}

		ctx := context.Background()
		response := that.Channel.Run(ctx, path.Join(util.GetProgramPath(), "strace"), args)

		if !response.Success {
			bin.PrintErrAndExit(response.Err)
		}
		bin.PrintOutputAndExit(response.Result.(string))
		return
	}
}

