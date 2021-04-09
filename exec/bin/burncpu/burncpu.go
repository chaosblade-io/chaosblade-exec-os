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
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/process"

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
	CpuCount     int    `name:"cpu-count" json:"cpu-count" yaml:"cpu-count" default:"${CPUNum}" help:"number of cpus"`
	CpuPercent   int    `name:"cpu-percent" json:"cpu-percent" yaml:"cpu-percent" default:"100" help:"percent of burn-cpu"`
	CpuProcessor int    `name:"cpu-processor" json:"cpu-processor" yaml:"cpu-processor" default:"0" help:"only used for identifying process of cpu burn"`

	// default arguments
	SlopePercent float64           `kong:"-"`
	Channel      channel.OsChannel `kong:"-"`
	// for test mock
	RunBurnCpu           func(ctx context.Context, cpuCount int, cpuPercent int, pidNeeded bool, processor string, climbTime int) int `kong:"-"`
	BindBurnCpuByTaskSet func(ctx context.Context, core string, pid int)                                                              `kong:"-"`
	CheckBurnCpu         func(ctx context.Context)                                                                                    `kong:"-"`
	StopBurnCpu          func() (bool, string)                                                                                        `kong:"-"`
}

func (that *BurnCPU) Assign() model.Worker {
	worker := &BurnCPU{Channel: channel.NewLocalChannel()}
	worker.RunBurnCpu = func(ctx context.Context, cpuCount int, cpuPercent int, pidNeeded bool, processor string, climbTime int) int {
		return worker.runBurnCpu(ctx, cpuCount, cpuPercent, pidNeeded, processor, climbTime)
	}
	worker.BindBurnCpuByTaskSet = func(ctx context.Context, core string, pid int) {
		worker.bindBurnCpuByTaskset(ctx, core, pid)
	}
	worker.CheckBurnCpu = func(ctx context.Context) {
		worker.checkBurnCpu(ctx)
	}
	worker.StopBurnCpu = func() (bool, string) {
		return worker.stopBurnCpu()
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

	if that.ClimbTime == 0 {
		that.SlopePercent = float64(that.CpuPercent)
	} else {
		var ticker *time.Ticker = time.NewTicker(1 * time.Second)
		that.SlopePercent = totalCpuPercent[0]
		var startPercent = float64(that.CpuPercent) - that.SlopePercent
		go func() {
			for range ticker.C {
				if that.SlopePercent < float64(that.CpuPercent) {
					that.SlopePercent += startPercent / float64(that.ClimbTime)
				} else if that.SlopePercent > float64(that.CpuPercent) {
					that.SlopePercent -= startPercent / float64(that.ClimbTime)
				}
			}
		}()
	}

	for i := 0; i < that.CpuCount; i++ {
		go func() {
			busy := int64(0)
			idle := int64(0)
			all := int64(10000000)
			dx := 0.0
			ds := time.Duration(0)
			for i := 0; ; i = (i + 1) % 1000 {
				startTime := time.Now().UnixNano()
				if i == 0 {
					dx = (that.SlopePercent - totalCpuPercent[0]) / otherCpuPercent
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

// startBurnCpu by invoke burnCpuBin with --nohup flag
func (that *BurnCPU) startBurnCpu() {
	ctx := context.Background()
	if that.CpuList != "" {
		that.CpuCount = 1
		cores := strings.Split(that.CpuList, ",")
		for _, core := range cores {
			pid := that.RunBurnCpu(ctx, that.CpuCount, that.CpuPercent, true, core, that.ClimbTime)
			that.BindBurnCpuByTaskSet(ctx, core, pid)
		}
	} else {
		that.RunBurnCpu(ctx, that.CpuCount, that.CpuPercent, false, "", that.ClimbTime)
	}
	that.CheckBurnCpu(ctx)
}

// runBurnCpu
func (that *BurnCPU) runBurnCpu(ctx context.Context, cpuCount int, cpuPercent int, pidNeeded bool, processor string, climbTime int) int {
	args := fmt.Sprintf(`%s --nohup --cpu-count %d --cpu-percent %d --climb-time %d`,
		path.Join(util.GetProgramPath(), that.Name()), cpuCount, cpuPercent, climbTime)
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
	return true, errs
}
