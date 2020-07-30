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
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/cgroups"
	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

var (
	burnCpuStart, burnCpuStop, burnCpuNohup bool
	cpuCount                                int
	cpuList                                 string
	cpuPercent                              int
	cpuProcessor                            string
)

func main() {
	flag.BoolVar(&burnCpuStart, "start", false, "start burn cpu")
	flag.BoolVar(&burnCpuStop, "stop", false, "stop burn cpu")
	flag.StringVar(&cpuList, "cpu-list", "", "CPUs in which to allow burning (1,3)")
	flag.BoolVar(&burnCpuNohup, "nohup", false, "nohup to run burn cpu")
	flag.IntVar(&cpuCount, "cpu-count", runtime.NumCPU(), "number of cpus")
	flag.IntVar(&cpuPercent, "cpu-percent", 100, "percent of burn-cpu")
	flag.StringVar(&cpuProcessor, "cpu-processor", "0", "only used for identifying process of cpu burn")
	flag.Parse()

	if cpuCount <= 0 || cpuCount > runtime.NumCPU() {
		cpuCount = runtime.NumCPU()
	}

	if burnCpuStart {
		startBurnCpu()
	} else if burnCpuStop {
		if success, errs := stopBurnCpuFunc(); !success {
			bin.PrintErrAndExit(errs)
		}
	} else if burnCpuNohup {
		burnCpu()
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
}

func burnCpu() {
	runtime.GOMAXPROCS(cpuCount)

	for i := 0; i < cpuCount; i++ {
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

var burnCpuBin = "chaos_burncpu"

var burnCpuCgroup = "/chaos_burncpu"

const cfsPeriodUs = uint64(200000)

const cfsQuotaUs = int64(2000)

var cl = channel.NewLocalChannel()

var stopBurnCpuFunc = stopBurnCpu

var runBurnCpuFunc = runBurnCpu

var bindBurnCpuFunc = bindBurnCpuByCpuset

var checkBurnCpuFunc = checkBurnCpu

var cgroupNewFunc = cgroupNew

// startBurnCpu by invoke burnCpuBin with --nohup flag
func startBurnCpu() {
	ctx := context.Background()
	if cpuPercent <= 0 || cpuPercent > 100 {
		cpuPercent = 100
	}
	if cpuList != "" {
		cpuCount = 1
		cores := strings.Split(cpuList, ",")
		realCores := len(cores)
		if realCores > runtime.NumCPU() {
			realCores = runtime.NumCPU()
		}
		control := cgroupNewFunc(realCores, cpuPercent)
		for _, core := range cores {
			pid := runBurnCpuFunc(ctx, cpuCount, true, core)
			if err := control.Add(cgroups.Process{Pid: pid}); err != nil {
				stopBurnCpuFunc()
				bin.PrintErrAndExit(fmt.Sprintf("Add pid to cgroup error, %v", err))
			}
		}
		bindBurnCpuFunc(control, cpuList)
	} else {
		pid := runBurnCpuFunc(ctx, cpuCount, true, "0")
		control := cgroupNewFunc(cpuCount, cpuPercent)
		if err := control.Add(cgroups.Process{Pid: pid}); err != nil {
			stopBurnCpuFunc()
			bin.PrintErrAndExit(fmt.Sprintf("Add pid to cgroup error, %v", err))
		}
	}
	checkBurnCpuFunc(ctx)
}

// runBurnCpu
func runBurnCpu(ctx context.Context, cpuCount int, pidNeeded bool, processor string) int {
	args := fmt.Sprintf(`%s --nohup --cpu-count %d`,
		path.Join(util.GetProgramPath(), burnCpuBin), cpuCount)

	if pidNeeded {
		args = fmt.Sprintf("%s --cpu-processor %s", args, processor)
	}
	args = fmt.Sprintf(`%s > /dev/null 2>&1 &`, args)
	response := cl.Run(ctx, "nohup", args)
	if !response.Success {
		stopBurnCpuFunc()
		bin.PrintErrAndExit(response.Err)
	}
	if pidNeeded {
		// parse pid
		newCtx := context.WithValue(context.Background(), channel.ProcessKey, fmt.Sprintf("cpu-processor %s", processor))
		pids, err := cl.GetPidsByProcessName(burnCpuBin, newCtx)
		if err != nil {
			stopBurnCpuFunc()
			bin.PrintErrAndExit(fmt.Sprintf("bind cpu core failed, cannot get the burning program pid, %v", err))
		}
		if len(pids) > 0 {
			// return the first one
			pid, err := strconv.Atoi(pids[0])
			if err != nil {
				stopBurnCpuFunc()
				bin.PrintErrAndExit(fmt.Sprintf("bind cpu core failed, get pid failed, pids: %v, err: %v", pids, err))
			}
			return pid
		}
	}
	return -1
}

// bindBurnCpu by taskset command
func bindBurnCpuByTaskset(ctx context.Context, core string, pid int) {
	response := cl.Run(ctx, "taskset", fmt.Sprintf("-a -cp %s %d", core, pid))
	if !response.Success {
		stopBurnCpuFunc()
		bin.PrintErrAndExit(response.Err)
	}
}

// bindBurnCpu by cpuset
func bindBurnCpuByCpuset(cgctrl cgroups.Cgroup, cpuList string) {
	if err := cgctrl.Update(&specs.LinuxResources{CPU: &specs.LinuxCPU{Cpus: cpuList}}); err != nil {
		stopBurnCpuFunc()
		bin.PrintErrAndExit(fmt.Sprintf("Bind core-list to cgroup error, %v", err))
	}
}

// checkBurnCpu
func checkBurnCpu(ctx context.Context) {
	time.Sleep(time.Second)
	// query process
	ctx = context.WithValue(ctx, channel.ProcessKey, "nohup")
	pids, _ := cl.GetPidsByProcessName(burnCpuBin, ctx)
	if pids == nil || len(pids) == 0 {
		bin.PrintErrAndExit(fmt.Sprintf("%s pid not found", burnCpuBin))
	}
}

// stopBurnCpu
func stopBurnCpu() (success bool, errs string) {
	// add grep nohup
	ctx := context.WithValue(context.Background(), channel.ProcessKey, "nohup")
	pids, _ := cl.GetPidsByProcessName(burnCpuBin, ctx)
	if pids == nil || len(pids) == 0 {
		return true, errs
	}
	response := cl.Run(ctx, "kill", fmt.Sprintf(`-9 %s`, strings.Join(pids, " ")))
	if !response.Success {
		return false, response.Err
	}

	//delete burnCpuCgroup
	control, err := cgroups.Load(cgroups.V1, cgroups.StaticPath(burnCpuCgroup))
	if err == nil {
		control.Delete()
	}
	return true, errs
}

//add a cgroup
func cgroupNew(cores int, percent int) cgroups.Cgroup {
	period := cfsPeriodUs
	quota := cfsQuotaUs * int64(cores) * int64(percent)
	control, err := cgroups.New(cgroups.V1, cgroups.StaticPath(burnCpuCgroup), &specs.LinuxResources{
		CPU: &specs.LinuxCPU{
			Period: &period,
			Quota:  &quota,
		},
	})
	if err != nil {
		stopBurnCpuFunc()
		bin.PrintErrAndExit(fmt.Sprintf("create cgroup error, %v", err))
	}
	return control
}
