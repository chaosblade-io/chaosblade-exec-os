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
	"github.com/containerd/cgroups"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/process"
	"go.uber.org/atomic"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	_ "go.uber.org/automaxprocs"
)

var (
	burnCpuStart, burnCpuStop, burnCpuNohup bool
	cpuCount, cpuPercent, climbTime         int
	slopePercent                            float64
	cpuList                                 string
	cpuProcessor                            string
	maxprocs                                int
)

func init() {
	maxprocs = runtime.GOMAXPROCS(-1)
}

func main() {
	flag.BoolVar(&burnCpuStart, "start", false, "start burn cpu")
	flag.BoolVar(&burnCpuStop, "stop", false, "stop burn cpu")
	flag.StringVar(&cpuList, "cpu-list", "", "CPUs in which to allow burning (1,3)")
	flag.BoolVar(&burnCpuNohup, "nohup", false, "nohup to run burn cpu")
	flag.IntVar(&climbTime, "climb-time", 0, "durations(s) to climb")
	flag.IntVar(&cpuCount, "cpu-count", maxprocs, "number of logic cpus")
	flag.IntVar(&cpuPercent, "cpu-percent", 100, "percent of burn-cpu")
	flag.StringVar(&cpuProcessor, "cpu-processor", "0", "only used for identifying process of cpu burn")
	bin.ParseFlagAndInitLog()

	if cpuCount <= 0 || cpuCount > maxprocs {
		cpuCount = maxprocs
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

func fileExists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}

func burnCpu() {
	var cpuCgroupPath = "/sys/fs/cgroup/cpu/"
	if !fileExists(cpuCgroupPath) {
		burnCpuForHost()
		return
	}

	// quota == -1 cpu limited by cpu.shares or cpuset.cpus
	_, quota := getPeriodAndQuota()
	if quota == -1 {
		burnCpuForHost()
		return
	}

	// quota==-1
	burnCpuForDocker()
	return
}

func burnCpuForHost() {

	runtime.GOMAXPROCS(cpuCount)

	var totalCpuPercent []float64
	var curProcess *process.Process
	var curCpuPercent float64
	var err error

	totalCpuPercent, err = cpu.Percent(time.Second, false)
	if err != nil {
		bin.PrintErrAndExit(err.Error())
	}

	curProcess, err = process.NewProcess(int32(os.Getpid()))
	if err != nil {
		bin.PrintErrAndExit(err.Error())
	}

	curCpuPercent, err = curProcess.CPUPercent()
	if err != nil {
		bin.PrintErrAndExit(err.Error())
	}

	otherCpuPercent := (100.0 - (totalCpuPercent[0] - curCpuPercent)) / 100.0
	go func() {
		t := time.NewTicker(3 * time.Second)
		for {
			select {
			// timer 3s
			case <-t.C:
				totalCpuPercent, err = cpu.Percent(time.Second, false)
				if err != nil {
					bin.PrintErrAndExit(err.Error())
				}

				curCpuPercent, err = curProcess.CPUPercent()
				if err != nil {
					bin.PrintErrAndExit(err.Error())
				}
				otherCpuPercent = (100.0 - (totalCpuPercent[0] - curCpuPercent)) / 100.0
			}
		}
	}()

	if climbTime == 0 {
		slopePercent = float64(cpuPercent)
	} else {
		var ticker *time.Ticker = time.NewTicker(1 * time.Second)
		slopePercent = totalCpuPercent[0]
		var startPercent = float64(cpuPercent) - slopePercent
		go func() {
			for range ticker.C {
				if slopePercent < float64(cpuPercent) {
					slopePercent += startPercent / float64(climbTime)
				} else if slopePercent > float64(cpuPercent) {
					slopePercent -= startPercent / float64(climbTime)
				}
			}
		}()
	}

	for i := 0; i < cpuCount; i++ {
		go func() {
			busy := int64(0)
			idle := int64(0)
			all := int64(10000000)
			dx := 0.0
			ds := time.Duration(0)
			for i := 0; ; i = (i + 1) % 1000 {
				startTime := time.Now().UnixNano()
				if i == 0 {
					dx = (slopePercent - totalCpuPercent[0]) / otherCpuPercent
					busy = busy + int64(dx*100000)
					if busy < 0 {
						busy = 0
					}
					idle = all - busy
					if idle < 0 {
						idle = 0
					}
					ds, _ = time.ParseDuration(strconv.FormatInt(idle, 10) + "ns")
				}
				for time.Now().UnixNano()-startTime < busy {
				}
				time.Sleep(ds)
				runtime.Gosched()
			}
		}()
	}
	select {}
}

func burnCpuForDocker() {

	//use automaxprocs auto set cpu
	//runtime.GOMAXPROCS(cpuCount)

	var usageLastSec atomic.Int64
	var preUsage, curUsage int64 // ns

	cgroup, err := cgroups.Load(cgroups.V1, cgroups.StaticPath("/"))
	if err != nil {
		stopBurnCpu()
		bin.PrintErrAndExit(err.Error())
	}

	getCPUUsage := func() int64 {
		stats, err := cgroup.Stat(cgroups.IgnoreNotExist)
		if err != nil {
			stopBurnCpu()
			bin.PrintErrAndExit(err.Error())
		}
		return int64(stats.CPU.Usage.Total)
	}

	preUsage = getCPUUsage()
	period, quota := getPeriodAndQuota()

	go func() {
		// timer 1s
		t := time.NewTicker(1 * time.Second)
		for {
			select {
			// timer 1s
			case <-t.C:
				curUsage = getCPUUsage()
				usageLastSec.Store(curUsage - preUsage)
				preUsage = curUsage
			}
		}
	}()

	time.Sleep(time.Second)

	var targetCPUTimeSlice atomic.Int64
	cpuPercentTimeSlice := int64(float64(1000000000/period) * float64(quota) * float64(cpuPercent) / 100)

	if climbTime == 0 {
		targetCPUTimeSlice.Store(cpuPercentTimeSlice)
	} else {
		var ticker *time.Ticker = time.NewTicker(1 * time.Second)
		targetCPUTimeSlice.Store(usageLastSec.Load())
		var startPercentTimeSlice = cpuPercentTimeSlice - targetCPUTimeSlice.Load()
		go func() {
			for range ticker.C {
				temp := targetCPUTimeSlice.Load()
				if temp < cpuPercentTimeSlice {
					targetCPUTimeSlice.Store(temp + startPercentTimeSlice/int64(climbTime))
				} else if temp > cpuPercentTimeSlice {
					targetCPUTimeSlice.Store(temp - startPercentTimeSlice/int64(climbTime))
				}
			}
		}()
	}

	for i := 0; i < cpuCount; i++ {
		go func() {
			busy := int64(0)
			idle := int64(0)
			all := int64(10000000)
			ds := time.Duration(0)
			// 1s分成100等分
			for i := 0; ; i = (i + 1) % 100 {
				startTime := time.Now().UnixNano()
				if i == 0 {
					dx := (targetCPUTimeSlice.Load() - usageLastSec.Load()) / 100
					busy = busy + dx
					if busy < 0 {
						busy = 0
					}
					idle = all - busy
					if idle < 0 {
						idle = 0
					}
					ds, _ = time.ParseDuration(strconv.FormatInt(idle, 10) + "ns")
				}
				for time.Now().UnixNano()-startTime < busy {
				}
				time.Sleep(ds)
				runtime.Gosched()
			}
		}()
	}
	select {}
}

var burnCpuBin = exec.BurnCpuBin

var cl = channel.NewLocalChannel()

var stopBurnCpuFunc = stopBurnCpu

var runBurnCpuFunc = runBurnCpu

var bindBurnCpuFunc = bindBurnCpuByTaskset

var checkBurnCpuFunc = checkBurnCpu

// startBurnCpu by invoke burnCpuBin with --nohup flag
func startBurnCpu() {
	ctx := context.Background()
	if cpuList != "" {
		cpuCount = 1
		cores := strings.Split(cpuList, ",")
		for _, core := range cores {
			pid := runBurnCpuFunc(ctx, cpuCount, cpuPercent, true, core, climbTime)
			bindBurnCpuFunc(ctx, core, pid)
		}
	} else {
		runBurnCpuFunc(ctx, cpuCount, cpuPercent, false, "", climbTime)
	}
	checkBurnCpuFunc(ctx)
}

// runBurnCpu
func runBurnCpu(ctx context.Context, cpuCount int, cpuPercent int, pidNeeded bool, processor string, climbTime int) int {
	args := fmt.Sprintf(`%s --nohup --cpu-count %d --cpu-percent %d --climb-time %d`,
		path.Join(util.GetProgramPath(), burnCpuBin), cpuCount, cpuPercent, climbTime)
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
	response := cl.Run(ctx, "taskset", fmt.Sprintf("-cp %s %d", core, pid))
	if !response.Success {
		stopBurnCpuFunc()
		bin.PrintErrAndExit(response.Err)
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
	return true, errs
}

func getPeriodAndQuota() (period int64, quota int64) {
	var cpuCgroupPath = "/sys/fs/cgroup/cpu/"
	quota, err := parseCgroupFile(cpuCgroupPath + "cpu.cfs_quota_us")
	if err != nil {
		stopBurnCpuFunc()
		bin.PrintErrAndExit(fmt.Sprintf("read cpu.cfs_quota_us failed, %v", err))
	}

	period, err = parseCgroupFile(cpuCgroupPath + "cpu.cfs_period_us")
	if err != nil {
		stopBurnCpu()
		bin.PrintErrAndExit(fmt.Sprintf("read cpu.cfs_period_us failed, %v", err))
	}

	return
}

func parseCgroupFile(path string) (int64, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, err
	}

	res, err := strconv.ParseInt(strings.Split(string(bytes), "\n")[0], 10, 64)
	if err != nil {
		return 0, err
	}

	return res, nil
}
