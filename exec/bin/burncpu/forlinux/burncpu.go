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

package burncpu

import (
	"context"
	"fmt"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/model"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/cgroups"
	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

// init registry provider to model.
func init() {
	model.Provide(new(BurnCPU))
}

type BurnCPU struct {
	BurnCpuStart bool   `name:"start" json:"start" yaml:"start" default:"false" help:"start burn cpu"`
	BurnCpuStop  bool   `name:"stop" json:"stop" yaml:"stop" default:"false" help:"stop burn cpu"`
	CpuList      string `name:"cpu-list" json:"cpu-list" yaml:"cpu-list" default:"" help:"CPUs in which to allow burning (1,3)"`
	BurnCpuNohup bool   `name:"nohup" json:"nohup" yaml:"nohup" default:"false" help:"nohup to run burn cpu"`
	ClimbTime    int    `name:"climb-time" json:"climb-time" yaml:"climb-time" default:"0" help:"durations(s) to climb"`
	CpuCount     int    `name:"cpu-count" json:"cpu-count" yaml:"cpu-count" default:"${runtime.NumCPU()}" help:"number of cpus"`
	CpuPercent   int    `name:"cpu-percent" json:"cpu-percent" yaml:"cpu-percent" default:"100" help:"percent of burn-cpu"`
	CpuProcessor int    `name:"cpu-processor" json:"cpu-processor" yaml:"cpu-processor" default:"0" help:"only used for identifying process of cpu burn"`
	// default arguments
	Channel channel.OsChannel `kong:"-"`
	// for test mock
	RunBurnCpu          func(ctx context.Context, cpuCount int, pidNeeded bool, processor string) int `kong:"-"`
	CheckBurnCpu        func(ctx context.Context)                                                     `kong:"-"`
	BindBurnCpuByCpuSet func(cgctrl cgroups.Cgroup, cpuList string)                                   `kong:"-"`
	StopBurnCpu         func() (success bool, errs string)                                            `kong:"-"`
	CGroupNew           func(cores int, percent int) cgroups.Cgroup                                   `kong:"-"`
}

func (that *BurnCPU) Assign() model.Worker {
	worker := &BurnCPU{Channel: channel.NewLocalChannel()}
	worker.RunBurnCpu = func(ctx context.Context, cpuCount int, pidNeeded bool, processor string) int {
		return worker.runBurnCpu(ctx, cpuCount, pidNeeded, processor)
	}
	worker.CheckBurnCpu = func(ctx context.Context) {
		worker.checkBurnCpu(ctx)
	}
	worker.BindBurnCpuByCpuSet = func(cgctrl cgroups.Cgroup, cpuList string) {
		worker.bindBurnCpuByCpuset(cgctrl, cpuList)
	}
	worker.StopBurnCpu = func() (success bool, errs string) {
		return worker.stopBurnCpu()
	}
	worker.CGroupNew = func(cores int, percent int) cgroups.Cgroup {
		return that.cgroupNew(cores, percent)
	}
	return worker
}

func (that *BurnCPU) Name() string {
	return exec.BurnCpuBin
}

func (that *BurnCPU) Exec() *spec.Response {
	if that.CpuCount <= 0 || that.CpuCount > runtime.NumCPU() {
		that.CpuCount = runtime.NumCPU()
	}

	if that.BurnCpuStart {
		that.startBurnCpu()
	} else if that.BurnCpuStop {
		if success, errs := that.StopBurnCpu(); !success {
			bin.PrintErrAndExit(errs)
		}
	} else if that.BurnCpuNohup {
		that.burnCpu()
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
	return spec.ReturnSuccess("")
}

func (that *BurnCPU) burnCpu() {
	runtime.GOMAXPROCS(that.CpuCount)

	for i := 0; i < that.CpuCount; i++ {
		go func() {
			for {
				for i := 0; i < 2147483647; i++ {
				}
				runtime.Gosched()
			}
		}()
	}
	select {} // wait forever
}

func (that *BurnCPU) BurnCpuCGroup() string {
	return "/" + that.Name()
}

const cfsPeriodUs = uint64(200000)

const cfsQuotaUs = int64(2000)


// startBurnCpu by invoke burnCpuBin with --nohup flag
func (that *BurnCPU) startBurnCpu() {
	ctx := context.Background()
	if that.CpuPercent <= 0 || that.CpuPercent > 100 {
		that.CpuPercent = 100
	}
	if that.CpuList != "" {
		that.CpuCount = 1
		cores := strings.Split(that.CpuList, ",")
		realCores := len(cores)
		if realCores > runtime.NumCPU() {
			realCores = runtime.NumCPU()
		}
		control := that.CGroupNew(realCores, that.CpuPercent)
		for _, core := range cores {
			pid := that.RunBurnCpu(ctx, that.CpuCount, true, core)
			if err := control.Add(cgroups.Process{Pid: pid}); err != nil {
				that.StopBurnCpu()
				bin.PrintErrAndExit(fmt.Sprintf("Add pid to cgroup error, %v", err))
			}
		}
		that.BindBurnCpuByCpuSet(control, that.CpuList)
	} else {
		pid := that.RunBurnCpu(ctx, that.CpuCount, true, "0")
		control := that.CGroupNew(that.CpuCount, that.CpuPercent)
		if err := control.Add(cgroups.Process{Pid: pid}); err != nil {
			that.StopBurnCpu()
			bin.PrintErrAndExit(fmt.Sprintf("Add pid to cgroup error, %v", err))
		}
	}
	that.CheckBurnCpu(ctx)
}

// runBurnCpu
func (that *BurnCPU) runBurnCpu(ctx context.Context, cpuCount int, pidNeeded bool, processor string) int {
	args := fmt.Sprintf(`%s --nohup --cpu-count %d`,
		path.Join(util.GetProgramPath(), that.Name()), cpuCount)

	if pidNeeded {
		args = fmt.Sprintf("%s --cpu-processor %s", args, processor)
	}
	args = fmt.Sprintf(`%s > /dev/null 2>&1 &`, args)
	response := that.Channel.Run(ctx, "nohup", args)
	if !response.Success {
		that.StopBurnCpu()
		bin.PrintErrAndExit(response.Err)
	}
	if pidNeeded {
		// parse pid
		newCtx := context.WithValue(context.Background(), channel.ProcessKey, fmt.Sprintf("cpu-processor %s", processor))
		pids, err := that.Channel.GetPidsByProcessName(that.Name(), newCtx)
		if err != nil {
			that.StopBurnCpu()
			bin.PrintErrAndExit(fmt.Sprintf("bind cpu core failed, cannot get the burning program pid, %v", err))
		}
		if len(pids) > 0 {
			// return the first one
			pid, err := strconv.Atoi(pids[0])
			if err != nil {
				that.StopBurnCpu()
				bin.PrintErrAndExit(fmt.Sprintf("bind cpu core failed, get pid failed, pids: %v, err: %v", pids, err))
			}
			return pid
		}
	}
	return -1
}

// bindBurnCpu by taskset command
func (that *BurnCPU) bindBurnCpuByTaskset(ctx context.Context, core string, pid int) {
	response := that.Channel.Run(ctx, "taskset", fmt.Sprintf("-a -cp %s %d", core, pid))
	if !response.Success {
		that.StopBurnCpu()
		bin.PrintErrAndExit(response.Err)
	}
}

// bindBurnCpu by cpuset
func (that *BurnCPU) bindBurnCpuByCpuset(cgctrl cgroups.Cgroup, cpuList string) {
	if err := cgctrl.Update(&specs.LinuxResources{CPU: &specs.LinuxCPU{Cpus: cpuList}}); err != nil {
		that.StopBurnCpu()
		bin.PrintErrAndExit(fmt.Sprintf("Bind core-list to cgroup error, %v", err))
	}
}

// checkBurnCpu
func (that *BurnCPU) checkBurnCpu(ctx context.Context) {
	time.Sleep(time.Second)
	// query process
	ctx = context.WithValue(ctx, channel.ProcessKey, "nohup")
	pids, _ := that.Channel.GetPidsByProcessName(that.Name(), ctx)
	if pids == nil || len(pids) == 0 {
		bin.PrintErrAndExit(fmt.Sprintf("%s pid not found", that.Name()))
	}
}

// stopBurnCpu
func (that *BurnCPU) stopBurnCpu() (success bool, errs string) {
	// add grep nohup
	ctx := context.WithValue(context.Background(), channel.ProcessKey, "nohup")
	pids, _ := that.Channel.GetPidsByProcessName(that.Name(), ctx)
	if pids == nil || len(pids) == 0 {
		return true, errs
	}
	response := that.Channel.Run(ctx, "kill", fmt.Sprintf(`-9 %s`, strings.Join(pids, " ")))
	if !response.Success {
		return false, response.Err
	}

	//delete burnCpuCgroup
	control, err := cgroups.Load(cgroups.V1, cgroups.StaticPath(that.BurnCpuCGroup()))
	if err == nil {
		control.Delete()
	}
	return true, errs
}

//add a cgroup
func (that *BurnCPU) cgroupNew(cores int, percent int) cgroups.Cgroup {
	period := cfsPeriodUs
	quota := cfsQuotaUs * int64(cores) * int64(percent)
	control, err := cgroups.New(cgroups.V1, cgroups.StaticPath(that.BurnCpuCGroup()), &specs.LinuxResources{
		CPU: &specs.LinuxCPU{
			Period: &period,
			Quota:  &quota,
		},
	})
	if err != nil {
		that.StopBurnCpu()
		bin.PrintErrAndExit(fmt.Sprintf("create cgroup error, %v", err))
	}
	return control
}
